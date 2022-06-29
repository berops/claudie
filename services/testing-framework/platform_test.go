package testingframework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"

	"io/fs"
	"io/ioutil"
	"testing"
)

type idInfo struct {
	id     string
	idType pb.IdType
}

const (
	testDir = "tests"

	maxTimeout     = 3600   // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 30     // seconds for one cycle of config check
	maxTimeoutSave = 60 * 6 // max allowed time for config to be found in the database
)

// TestPlatform will start all the test cases specified in tests directory
func TestPlatform(t *testing.T) {
	utils.InitLog("testing-framework", "GOLANG_LOG")
	c := clientConnection()
	log.Info().Msg("----Starting the tests----")

	// loop through the directory and list files inside
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test sets: %v", err)
	}

	// save all the test set paths
	var setNames []string
	for _, f := range files {
		if f.IsDir() {
			log.Info().Msgf("Found test set: %s", f.Name())
			setNames = append(setNames, f.Name())
		}
	}

	// retrieve namespace from ENV
	namespace := os.Getenv("NAMESPACE")

	// apply the test sets
	for _, path := range setNames {
		err := applyTestSet(path, namespace, c)
		if err != nil {
			t.Logf("Error while processing %s : %v", path, err)
			t.Fail()
		}
	}
}

// clientConnection will return new client connection to Context-box
func clientConnection() pb.ContextBoxServiceClient {
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
}

// applyTestSet function will apply test set sequentially to a platform
func applyTestSet(setName, namespace string, c pb.ContextBoxServiceClient) error {
	done := make(chan string)
	idInfo := idInfo{id: "", idType: -1}

	pathToTestSet := filepath.Join(testDir, setName)
	log.Info().Msgf("Working on the test set: %s", pathToTestSet)

	manifestFiles, err := ioutil.ReadDir(pathToTestSet)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test manifests: %v", err)
	}

	for _, manifest := range manifestFiles {
		// create a path and read the file
		manifestPath := filepath.Join(pathToTestSet, manifest.Name())
		yamlFile, err := ioutil.ReadFile(manifestPath)
		if err != nil {
			log.Error().Msgf("Error while reading the manifest %s : %v", manifestPath, err)
			return err
		}
		manifestName, err := getManifestName(yamlFile)
		if err != nil {
			log.Error().Msgf("Error while getting the manifest name from %s : %v", manifestPath, err)
			return err
		}

		if namespace != "" {
			err = clusterTesting(yamlFile, setName, pathToTestSet, namespace, manifestName, c)
			idInfo.id = manifestName
			idInfo.idType = pb.IdType_NAME
			if err != nil {
				log.Error().Msgf("Error while applying manifest %s : %v", manifest.Name(), err)
				return err
			}
		} else {
			idInfo.id, err = localTesting(manifest, yamlFile, manifestName, c)
			idInfo.idType = pb.IdType_HASH
			if err != nil {
				log.Error().Msgf("Error while applying manifest %s : %v", manifest.Name(), err)
				return err
			}
		}

		go configChecker(done, c, pathToTestSet, manifest.Name(), idInfo)
		// wait until test config has been processed
		if res := <-done; res != "ok" {
			log.Error().Msg(res)
			return fmt.Errorf(res)
		}
	}

	// clean up
	log.Info().Msgf("Deleting the clusters from test set: %s", pathToTestSet)

	//delete secret from cluster
	if namespace != "" {
		err = deleteSecret(setName, namespace)
		if err != nil {
			log.Error().Msgf("Error while deleting the secret from %s : %v", pathToTestSet, err)
			return err
		}
	} else {
		// delete config from database
		err = cbox.DeleteConfig(c, idInfo.id, pb.IdType_HASH)
		if err != nil {
			log.Error().Msgf("Error while deleting the clusters from test set %s : %v", pathToTestSet, err)
			return err
		}
	}

	return nil
}

// configChecker function will check if the config has been applied every 30s
func configChecker(done chan string, c pb.ContextBoxServiceClient, testSetName, manifestName string, idInfo idInfo) {
	counter := 1
	for {
		elapsedSec := counter * sleepSec
		config, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{
			Id:   idInfo.id,
			Type: idInfo.idType,
		})
		if err != nil {
			log.Fatal().Msg(fmt.Sprintf("Got error while waiting for config to finish: %v", err))
		}
		if config != nil {
			if len(config.Config.ErrorMessage) > 0 {
				emsg := config.Config.ErrorMessage
				log.Error().Msg(emsg)
				done <- emsg
				return
			}

			// if checksums are equal, the config has been processed by claudie
			if checksumsEqual(config.Config.MsChecksum, config.Config.CsChecksum) && checksumsEqual(config.Config.CsChecksum, config.Config.DsChecksum) {
				// test longhorn deployment
				err := testLonghornDeployment(config)
				if err != nil {
					log.Fatal().Msg(err.Error())
					done <- err.Error()
				}
				log.Info().Msgf("Manifest %s from %s is done...", manifestName, testSetName)
				break
			}
		}
		if elapsedSec >= maxTimeout {
			emsg := fmt.Sprintf("Test took too long... Aborting on timeout %d seconds", maxTimeout)
			log.Fatal().Msg(emsg)
			done <- emsg
			return
		}
		time.Sleep(time.Duration(sleepSec) * time.Second)
		counter++
		log.Info().Msgf("Waiting for %s to from %s finish... [ %ds elapsed ]", manifestName, testSetName, elapsedSec)
	}
	// send signal that config has been processed, unblock the applyTestSet
	done <- "ok"
}

// checksumsEq will check if two checksums are equal
func checksumsEqual(checksum1 []byte, checksum2 []byte) bool {
	if len(checksum1) > 0 && len(checksum2) > 0 && bytes.Equal(checksum1, checksum2) {
		return true
	} else {
		return false
	}
}

// clusterTesting will perform actions needed for testing framework to function in k8s cluster deployment
// this option is only used when NAMESPACE env var has been found
// this option is testing the whole claudie
func clusterTesting(yamlFile []byte, setName, pathToTestSet, namespace, manifestName string, c pb.ContextBoxServiceClient) error {
	// get the id from manifest file
	id, err := getManifestName(yamlFile)
	idType := pb.IdType_NAME
	if err != nil {
		log.Error().Msgf("Error while getting an id for %s : %v", manifestName, err)
		return err
	}

	if err != nil {
		return err
	}
	err = manageSecret(yamlFile, pathToTestSet, setName, namespace)
	if err != nil {
		log.Error().Msgf("Error while creating/editing a secret : %v", err)
		return err
	}
	err = checkIfManifestSaved(id, idType, c)
	if err != nil {
		return err
	}
	return nil
}

// localTesting will perform actions needed for testing framework to function in local deployment
// this option is only used when NAMESPACE env var has NOT been found
// this option is NOT testing the whole claudie (the frontend is omitted from workflow)
func localTesting(manifest fs.FileInfo, yamlFile []byte, manifestName string, c pb.ContextBoxServiceClient) (string, error) {
	// testing locally - NOT TESTING THE FRONTEND!
	id, err := cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
		Config: &pb.Config{
			Name:     manifestName,
			Manifest: string(yamlFile),
		},
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// checkIfManifestSaved function will wait until the manifest has been picked up from the secret by the frontend component and
// that it has been saved in database; throws error after set amount of time
func checkIfManifestSaved(configID string, idType pb.IdType, c pb.ContextBoxServiceClient) error {
	counter := 1
	// wait for the secret to be saved in the database and check if the secret has been updated with the new manifest
	for {
		time.Sleep(20 * time.Second)
		elapsedSec := counter * 20
		log.Info().Msgf("Waiting for secret to be picked up by the frontend... [ %ds elapsed...]", elapsedSec)
		counter++
		config, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{
			Id:   configID,
			Type: idType,
		})
		if err == nil {
			// if manifest checksum != desired state checksum -> the manifest has been updated
			if !checksumsEqual(config.Config.MsChecksum, config.Config.CsChecksum) || !checksumsEqual(config.Config.CsChecksum, config.Config.DsChecksum) {
				log.Info().Msgf("Manifest has been saved...")
				break
			} else {
				if elapsedSec > maxTimeoutSave {
					return fmt.Errorf("The secret has not been picked up by the frontend in time, aborting...")
				}
			}
		} else if elapsedSec > maxTimeoutSave {
			return fmt.Errorf("The secret has not been picked up by the frontend in time, aborting...")
		}
	}
	return nil
}
