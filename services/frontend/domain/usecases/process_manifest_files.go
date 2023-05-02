package usecases

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
)

// ProcessManifestFiles processes the manifest files concurrently. If an error occurs while the file
// is being processed, it's skipped and the function continues with the next one until all files are
// processed. Nothing is done with those files for which errors occurred, they'll be skipped until either
// corrected or deleted.
func (u *Usecases) ProcessManifestFiles(manifestDir string) error {
	manifestFiles, err := os.ReadDir(manifestDir)
	if err != nil {
		return fmt.Errorf("Failed to read manifest files from dir %q: %w", manifestDir, err)
	}

	configs, err := u.ContextBox.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("Failed to retrieve manifests details from context-box: %w", err)
	}

	log.Debug().Msgf("%d configs present in database | %d configs in %v", len(configs), len(manifestFiles), manifestDir)

	type ManifestProcessingResult struct {
		unmarshalledManifest *manifest.Manifest
		rawManifestData      []byte
		manifestFilepath     string
		processingError      error
	}

	manifestProcessingResultsChan := make(chan *ManifestProcessingResult, len(manifestFiles))

	waitGroup := sync.WaitGroup{}

	for _, manifestFile := range manifestFiles {
		waitGroup.Add(1)

		// Process each of the files concurrently in a separate go-routine skipping over files for which
		// an error occurs.
		// By processing, we mean reading, unmarshalling and validating the claudie manifest
		go func(manifestFile fs.DirEntry) {
			defer waitGroup.Done()

			manifestFilepath := filepath.Join(manifestDir, manifestFile.Name())

			var (
				rawManifestData      []byte
				unmarshalledManifest *manifest.Manifest
				processingError      error = nil
			)

			defer func() {
				manifestProcessingResultsChan <- &ManifestProcessingResult{
					unmarshalledManifest: unmarshalledManifest,
					rawManifestData:      rawManifestData,
					manifestFilepath:     manifestFilepath,
					processingError:      processingError,
				}
			}()

			if rawManifestData, processingError = os.ReadFile(manifestFilepath); processingError != nil {
				return
			}

			if processingError = yaml.Unmarshal(rawManifestData, &unmarshalledManifest); processingError != nil {
				return
			}

			processingError = unmarshalledManifest.Validate()
		}(manifestFile)
	}

	go func() {
		waitGroup.Wait()

		close(manifestProcessingResultsChan)
	}()

	// Collect processing results of manifest files which were processed successfully
	for manifestProcessingResult := range manifestProcessingResultsChan {
		var manifestName = manifestProcessingResult.unmarshalledManifest.Name
		var isConfigRemoved bool

		// Remove the config from configs.
		// After the for loop finishes, the configs variable will contain only those configs which represent
		// deleted manifest files.
		configs, isConfigRemoved = removeConfig(configs, manifestName)

		if manifestProcessingResult.processingError != nil {
			log.Err(manifestProcessingResult.processingError).
				Msgf("Skipping over processing file %v", manifestProcessingResult.manifestFilepath)

			continue
		}

		config := &pb.Config{
			Name:     manifestName,
			Manifest: string(manifestProcessingResult.rawManifestData),
		}

		err = u.ContextBox.SaveConfig(config)
		if err != nil {
			log.Err(err).Str("project", manifestName).Msgf("Failed to save config")
			continue
		}

		log.Info().Msgf("Details of the manifest file %s has been saved to context-box database", manifestProcessingResult.manifestFilepath)

		// if the config is not in the context-box DB we start to track it.
		if !isConfigRemoved {
			for _, k8sCluster := range manifestProcessingResult.unmarshalledManifest.Kubernetes.Clusters {
				if _, ok := u.inProgress.Load(k8sCluster.Name); !ok {
					u.inProgress.Store(k8sCluster.Name, config)
				}
			}
		}
	}

	// The configs variable now contains only those configs which represent deleted manifests.
	// Loop over each config and request the context-box microservice to delete the config from its database as well.
	for _, config := range configs {
		if _, isConfigBeingDeleted := u.configsBeingDeleted.Load(config.Id); isConfigBeingDeleted {
			continue
		}
		u.configsBeingDeleted.Store(config.Id, nil)

		for _, k8sCluster := range config.GetCurrentState().GetClusters() {
			if _, ok := u.inProgress.Load(k8sCluster.ClusterInfo.Name); !ok {
				u.inProgress.Store(k8sCluster.ClusterInfo.Name, config)
			}
		}

		go func(config *pb.Config) {
			log.Info().
				Str("project", config.Name).
				Msgf("Deleting config %v from context-box DB", config.Id)

			err := u.ContextBox.DeleteConfig(config.Id)
			if err != nil {
				log.Err(err).
					Str("project", config.Name).
					Msgf("Failed to delete config %s from MongoDB", config.Id)
			}

			u.configsBeingDeleted.Delete(config.Id)
		}(config)
	}

	return nil
}

// removeConfig filters out the config representing the manifest with
// the specified name from the configs slice. If not present the original slice is
// returned.
func removeConfig(configs []*pb.Config, manifestName string) ([]*pb.Config, bool) {
	var isConfigFound bool = false

	for index, config := range configs {
		if config.Name == manifestName {
			configs = append(configs[0:index], configs[index+1:]...)
			isConfigFound = true

			return configs, isConfigFound
		}
	}

	return configs, isConfigFound
}
