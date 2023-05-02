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
		return fmt.Errorf("failed to read manifest files from dir %q: %w", manifestDir, err)
	}

	configs, err := u.ContextBox.GetAllConfigs()
	if err != nil {
		return fmt.Errorf("failed to retrieve manifests details from context-box: %w", err)
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
			var (
				rawManifestData      []byte
				unmarshalledManifest *manifest.Manifest
				processingError      error = nil
			)
			defer waitGroup.Done()

			manifestFilepath := filepath.Join(manifestDir, manifestFile.Name())

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
			if processingError = yaml.Unmarshal(rawManifestData, unmarshalledManifest); processingError != nil {
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
		var manifestName string
		var isConfigRemoved bool

		// Remove the config from configs slice.
		// After the for loop finishes, the configs variable will contain only those configs which represent
		// deleted manifest files -> configs which needs to be deleted.
		configs, isConfigRemoved = removeConfig(configs, manifestProcessingResult.manifestFilepath)
		// Check for the error first, before referencing any variables.
		if manifestProcessingResult.processingError != nil {
			log.Error().Msgf("Skipping over file %v due to processing error : %v", manifestProcessingResult.manifestFilepath, manifestProcessingResult.processingError)
			continue
		}
		manifestName = manifestProcessingResult.unmarshalledManifest.Name

		config := &pb.Config{
			Name:             manifestName,
			ManifestFileName: manifestProcessingResult.manifestFilepath,
			Manifest:         string(manifestProcessingResult.rawManifestData),
		}

		err = u.ContextBox.SaveConfig(config)
		if err != nil {
			log.Error().Msgf("Failed to save config %v due to error : %v", manifestName, err)
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
			log.Info().Msgf("Deleting config %v from context-box DB", config.Id)

			err := u.ContextBox.DeleteConfig(config.Id)
			if err != nil {
				log.Error().Msgf("Failed to delete config %s of manifest %s : %v", config.Id, config.Name, err)
			}

			u.configsBeingDeleted.Delete(config.Id)
		}(config)
	}

	return nil
}

// removeConfig filters out the config representing the manifest with
// the specified path from the configs slice. If element removed from slice, new slice
// is returned together with value true. If element was not found in slice,
// original slice is returned together with value false.
func removeConfig(configs []*pb.Config, manifestPath string) ([]*pb.Config, bool) {
	for index, config := range configs {
		if config.ManifestFileName == manifestPath {
			configs = append(configs[0:index], configs[index+1:]...)
			return configs, true
		}
	}
	return configs, false
}
