package usecases

import (
	"claudie/proto/generated"
	"claudie/shared/manifest"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ProcessManifestFiles processes the manifest files concurrently. If an error occurs while the file
// is being processed it's skipped and continues with the next one until all are processed. Nothing
// is done with files for which an error occurred, they'll be skipped until either corrected or
// deleted.
func(u *Usecases) ProcessManifestFiles(manifestDir string) error {

	manifestFiles, err := os.ReadDir(manifestDir)
	if err != nil {
		return fmt.Errorf("Failed to read manifest files from dir %q: %w", manifestDir, err)}

	configList, err := u.ContextBox.GetConfigList( )
	if err != nil {
		return fmt.Errorf("Failed to retrieve manifests details from context-box: %w", err)}

	log.Debug( ).Msgf("%d manifests details in database | %d manifest files in %v", len(configList), len(manifestFiles), manifestDir)

	manifestProcessingResultsChan := make(chan *ManifestProcessingResult, len(manifestFiles))

	waitGroup := sync.WaitGroup{ }

	for _, manifestFile := range manifestFiles {
		waitGroup.Add(1)

		// Process each of the files concurrently in a separate go-routine skipping over file for which
		// an error occurs.
		go func(manifestFile fs.DirEntry) {
			defer waitGroup.Done( )

			manifestFilepath := filepath.Join(manifestDir, manifestFile.Name( ))

			var (
				rawManifestData []byte
				manifest manifest.Manifest
				processingError error
			)

			defer func( ) {
				manifestProcessingResultsChan <- &ManifestProcessingResult{
					manifestName: manifest.Name,
					rawManifestData: rawManifestData,
					manifestFilepath: manifestFilepath,
					processingError: processingError,
				}
			}( )

			if rawManifestData, processingError = os.ReadFile(manifestFilepath); err != nil {
				return}

			if processingError= yaml.Unmarshal(rawManifestData, &manifest); processingError != nil {
				return}
		}(manifestFile)
	}

	go func( ) {
		waitGroup.Wait( )

		close(manifestProcessingResultsChan)
	}( )

	// Collect results of manifest files which were processed successfully
	for manifestProcessingResult := range manifestProcessingResultsChan {

		configList= removeConfig(configList, manifestProcessingResult.manifestName)

		if manifestProcessingResult.processingError != nil {
			log.Error( ).Msgf("Skipping over file %v due to processing error : %v", manifestProcessingResult.manifestFilepath, manifestProcessingResult.processingError)
			continue
		}

		err= u.ContextBox.SaveConfig(
			&generated.Config{
				Name: manifestProcessingResult.manifestName,
				Content: string(manifestProcessingResult.rawManifestData),
			},
		)
		if err != nil {
			log.Error( ).Msgf("Skipped saving manifest-details %v due to error : %v", manifestProcessingResult.manifestName, err)
			continue
		}

		log.Info( ).Msgf("Details of manifest file %s has been saved to context-box database", manifestProcessingResult.manifestFilepath)
	}

	// threadSafeMap is a go-routine safe map that stores id of manifests-details that are being currently deleted
	// to avoid having multiple go-routines deleting the same manifest-details from the database.
	var threadSafeMap sync.Map

	for _, config := range configList {
		if _, isConfigBeingDeleted := threadSafeMap.Load(config.Id); isConfigBeingDeleted {
			continue}

		threadSafeMap.Store(config.Id, nil)

		go func(config *generated.Config) {
			log.Info( ).Msgf("Deleting manifest-details %v from context-box DB", config.Id)

			if err := u.ContextBox.DeleteConfig(config.Id); err != nil {
				log.Error( ).Msgf("Failed to delete manifest-details %s of manifest %s : %v", config.Id, config.Name, err)}

			threadSafeMap.Delete(config.Id)
		}(config)
	}

	return nil

}

// removeConfig filters out the manifest-details of the manifest with
// the specified name from the configList slice. If not present the original slice is
// returned.
func removeConfig(configList []*generated.Config, manifestName string) []*generated.Config {

	for index, config := range configList {
		if config.Name == manifestName {
			configList= append(configList[0:index], configList[index+1:]...)
			break
		}
	}

	return configList
}

type ManifestProcessingResult struct {

	manifestName string
	rawManifestData []byte
	manifestFilepath string
	processingError error
}