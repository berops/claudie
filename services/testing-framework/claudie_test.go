package testingframework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	cbox "github.com/Berops/claudie/services/context-box/client"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"testing"
)

type idInfo struct {
	id     string
	idType pb.IdType
}

const (
	testDir = "test-sets"

	maxTimeout     = 8000    // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 30      // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

// TestClaudie will start all the test cases specified in tests directory
func TestClaudie(t *testing.T) {
	utils.InitLog("testing-framework")
	c, cc := clientConnection()
	defer func() {
		err := cc.Close()
		if err != nil {
			log.Error().Msgf("error while closing client connection : %v", err)
		}
	}()
	log.Info().Msg("---- Starting the tests ----")

	// loop through the directory and list files inside
	files, err := os.ReadDir(testDir)
	if err != nil {
		log.Error().Msgf("Error while trying to read test sets: %v", err)
		t.Error(err)
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
	namespace := envs.Namespace

	// apply the test sets
	var errGroup errgroup.Group
	for _, path := range setNames {
		func(path, namespace string, c pb.ContextBoxServiceClient) {
			errGroup.Go(func() error {
				err := applyTestSet(path, namespace, c)
				if err != nil {
					return fmt.Errorf("error in test set %s : %w", path, err)
				}
				return nil
			})
		}(path, namespace, c)
	}
	err = errGroup.Wait()
	if err != nil {
		log.Error().Msgf("Error in one of the test sets : %v", err)
		t.Error(err)
	}
}

// clientConnection will return new client connection to Context-box
func clientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Msgf("Failed to create client connection to context-box : %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c, cc
}

// applyTestSet function will apply test set sequentially to Claudie
func applyTestSet(setName, namespace string, c pb.ContextBoxServiceClient) error {
	idInfo := idInfo{id: "", idType: -1}

	pathToTestSet := filepath.Join(testDir, setName)
	log.Info().Msgf("Working on the test set:%s", pathToTestSet)

	manifestFiles, err := os.ReadDir(pathToTestSet)
	if err != nil {
		return fmt.Errorf("error while trying to read test manifests in %s : %v", pathToTestSet, err)
	}

	for _, manifest := range manifestFiles {
		// https://github.com/Berops/claudie/pull/243#issuecomment-1218237412
		if manifest.IsDir() || manifest.Name()[0:1] == "." {
			continue
		}

		// create a path and read the file
		manifestPath := filepath.Join(pathToTestSet, manifest.Name())
		yamlFile, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("error while reading the manifest %s : %v", manifestPath, err)
		}
		manifestName, err := getManifestName(yamlFile)
		if err != nil {
			return fmt.Errorf("error while getting the manifest name from %s : %v", manifestPath, err)
		}

		if namespace != "" {
			err = clusterTesting(yamlFile, setName, pathToTestSet, namespace, manifestName, c)
			idInfo.id = manifestName
			idInfo.idType = pb.IdType_NAME
			if err != nil {
				return fmt.Errorf("error while applying manifest %s : %v", manifest.Name(), err)
			}
		} else {
			idInfo.id, err = localTesting(yamlFile, manifestName, c)
			idInfo.idType = pb.IdType_HASH
			if err != nil {
				return fmt.Errorf("error while applying manifest %s : %v", manifest.Name(), err)
			}
		}
		// wait until test config has been processed
		if err := configChecker(c, pathToTestSet, manifest.Name(), idInfo); err != nil {
			return fmt.Errorf("Error while monitoring %s : %w", pathToTestSet, err)
		}
		log.Info().Msgf("Manifest %s from %s is done...", manifestName, pathToTestSet)
	}

	// clean up
	log.Info().Msgf("Deleting the infra from test set %s", pathToTestSet)

	//delete secret from cluster
	if namespace != "" {
		if err = deleteSecret(setName, namespace); err != nil {
			return fmt.Errorf("error while deleting the secret %s from %s : %v", pathToTestSet, namespace, err)
		}
	} else {
		// delete config from database
		if err = cbox.DeleteConfig(c, idInfo.id, pb.IdType_HASH); err != nil {
			return fmt.Errorf("error while deleting the manifest from test set %s : %v", pathToTestSet, err)
		}
	}

	return nil
}

// configChecker function will check if the config has been applied every 30s
func configChecker(c pb.ContextBoxServiceClient, testSetName, manifestName string, idInfo idInfo) error {
	counter := 1
	for {
		elapsedSec := counter * sleepSec
		config, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{
			Id:   idInfo.id,
			Type: idInfo.idType,
		})
		if err != nil {
			return fmt.Errorf("error while waiting for config to finish: %w", err)
		}
		if config != nil {
			if len(config.Config.ErrorMessage) > 0 {
				return fmt.Errorf("error while checking config %s : %w", config.Config.Name, err)
			}

			// if checksums are equal, the config has been processed by claudie
			if checksumsEqual(config.Config.MsChecksum, config.Config.CsChecksum) && checksumsEqual(config.Config.CsChecksum, config.Config.DsChecksum) {
				// test longhorn deployment
				err := testLonghornDeployment(config)
				if err != nil {
					return fmt.Errorf("error while checking the longhorn deployment for %s : %w", config.Config.Name, err)
				}
				//manifest is done
				return nil
			}
		}
		if elapsedSec >= maxTimeout {
			return fmt.Errorf("Test took too long... Aborting after %d seconds", maxTimeout)
		}
		time.Sleep(time.Duration(sleepSec) * time.Second)
		counter++
		log.Info().Msgf("Waiting for %s to from %s finish... [ %ds elapsed ]", manifestName, testSetName, elapsedSec)
	}
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
		return fmt.Errorf("error while getting an id for %s : %v", manifestName, err)
	}

	if err = applySecret(yamlFile, pathToTestSet, setName, namespace); err != nil {
		return fmt.Errorf("error while applying a secret for %s : %v", setName, err)
	}
	log.Info().Msgf("Secret for config with id %s has been saved...", id)

	if err = checkIfManifestSaved(id, idType, c); err != nil {
		return fmt.Errorf("error while checking if  with id %s is saved : %w", id, err)
	}
	log.Info().Msgf("Manifest for config with id %s has been saved...", id)
	return nil
}

// localTesting will perform actions needed for testing framework to function in local deployment
// this option is only used when NAMESPACE env var has NOT been found
// this option is NOT testing the whole claudie (the frontend is omitted from workflow)
func localTesting(yamlFile []byte, manifestName string, c pb.ContextBoxServiceClient) (string, error) {
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
	log.Info().Msgf("Manifest for config with id %s has been saved...", id)
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
		log.Info().Msgf("Waiting for secret for config with id %s to be picked up by the frontend... [ %ds elapsed...]", configID, elapsedSec)
		counter++
		config, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{
			Id:   configID,
			Type: idType,
		})
		if err == nil {
			// if manifest checksum != desired state checksum -> the manifest has been updated
			if !checksumsEqual(config.Config.MsChecksum, config.Config.CsChecksum) || !checksumsEqual(config.Config.CsChecksum, config.Config.DsChecksum) {
				return nil
			} else {
				if elapsedSec > maxTimeoutSave {
					return fmt.Errorf("The secret for config with id %s has not been picked up by the frontend in time, aborting...", configID)
				}
			}
		} else if elapsedSec > maxTimeoutSave {
			return fmt.Errorf("The secret for config with id %s has not been picked up by the frontend in time, aborting...", configID)
		}
	}
}
