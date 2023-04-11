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

	manifestProcessingResultsChan := make(chan *ManifestProcessingResult, len(manifestFiles))

	waitGroup := sync.WaitGroup{}

	for _, manifestFile := range manifestFiles {
		waitGroup.Add(1)

		// Process each of the files concurrently in a separate go-routine skipping over files for which
		// an error occurs.
		// By processing, we mean unmarshalling the claudie manifest
		go func(manifestFile fs.DirEntry) {
			defer waitGroup.Done()

			manifestFilepath := filepath.Join(manifestDir, manifestFile.Name())

			var (
				rawManifestData []byte
				manifest        manifest.Manifest
				processingError error
			)

			defer func() {
				manifestProcessingResultsChan <- &ManifestProcessingResult{
					manifestName:     manifest.Name,
					rawManifestData:  rawManifestData,
					manifestFilepath: manifestFilepath,
					processingError:  processingError,
				}
			}()

			if rawManifestData, processingError = os.ReadFile(manifestFilepath); err != nil {
				return
			}

			if processingError = yaml.Unmarshal(rawManifestData, &manifest); processingError != nil {
				return
			}

			manifest.Validate()
		}(manifestFile)
	}

	go func() {
		waitGroup.Wait()

		close(manifestProcessingResultsChan)
	}()

	// Collect processing results of manifest files which were processed successfully
	for manifestProcessingResult := range manifestProcessingResultsChan {
		// TODO: review
		if manifestProcessingResult.processingError != nil {
			log.Error().Msgf("Skipping over file %v due to processing error : %v", manifestProcessingResult.manifestFilepath, manifestProcessingResult.processingError)
			continue
		}

		configs = removeConfig(configs, manifestProcessingResult.manifestName)

		err = u.ContextBox.SaveConfig(
			&pb.Config{
				Name:     manifestProcessingResult.manifestName,
				Manifest: string(manifestProcessingResult.rawManifestData),
			},
		)
		if err != nil {
			log.Error().Msgf("Skipped saving config %v due to error : %v", manifestProcessingResult.manifestName, err)
			continue
		}

		log.Info().Msgf("Details of manifest file %s has been saved to context-box database", manifestProcessingResult.manifestFilepath)
	}

	// threadSafeMap is a go-routine safe map that stores id of configs that are being currently deleted
	// to avoid having multiple go-routines deleting the same configs from MongoDB (of contextBox microservice).
	var threadSafeMap sync.Map

	for _, config := range configs {
		if _, isConfigBeingDeleted := threadSafeMap.Load(config.Id); isConfigBeingDeleted {
			continue
		}

		threadSafeMap.Store(config.Id, nil)

		go func(config *pb.Config) {
			log.Info().Msgf("Deleting config %v from context-box DB", config.Id)

			err := u.ContextBox.DeleteConfig(config.Id)
			if err != nil {
				log.Error().Msgf("Failed to delete config %s of manifest %s : %v", config.Id, config.Name, err)
			}

			threadSafeMap.Delete(config.Id)
		}(config)
	}

	return nil

}

// removeConfig filters out the config representing the manifest with
// the specified name from the configs slice. If not present the original slice is
// returned.
func removeConfig(configs []*pb.Config, manifestName string) []*pb.Config {

	for index, config := range configs {
		if config.Name == manifestName {
			configs = append(configs[0:index], configs[index+1:]...)
			break
		}
	}

	return configs
}

type ManifestProcessingResult struct {
	manifestName     string
	rawManifestData  []byte
	manifestFilepath string
	processingError  error
}
