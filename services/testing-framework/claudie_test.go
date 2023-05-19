package testingframework

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"testing"
)

const (
	testDir = "test-sets"
)

var (
	// get env var from runtime directly so we do not pollute original envs package by unnecessary variables
	autoCleanUpFlag = os.Getenv("AUTO_CLEAN_UP")
)

// TestClaudie will start all the test cases specified in tests directory
func TestClaudie(t *testing.T) {
	utils.InitLog("testing-framework")
	group := errgroup.Group{}
	ctx, cancel := context.WithCancel(context.Background())

	// start goroutine to check for SIGTERM
	group.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
			//if error is due to ctx being cancel, return nil
			if errors.Is(err, context.Canceled) {
				return nil
			}
		case sig := <-ch:
			log.Warn().Msgf("Received signal %v", sig)
			err = errors.New("interrupt signal")
			cancel()
		}

		return err
	})

	// start E2E tests in separate goroutines
	group.Go(func() error {
		// cancel the context so monitoring goroutine (SIGTERM) will exit
		defer cancel()
		// return error from testClaudie(), if any
		return testClaudie(ctx)
	})

	// wait for either test to finish or interrupt signal to occur
	// return either error from ctx, or error from testClaudie()
	if err := group.Wait(); err != nil {
		t.Error(err)
	}
}

func testClaudie(ctx context.Context) error {
	c, cc := clientConnection()
	defer func() {
		err := cc.Close()
		if err != nil {
			log.Err(err).Msgf("error while closing client connection")
		}
	}()
	log.Info().Msg("---- Starting the tests ----")

	// loop through the directory and list files inside
	files, err := os.ReadDir(testDir)
	if err != nil {
		return fmt.Errorf("error while trying to read test sets: %w", err)
	}

	// save all the test set paths
	var basicSets, autoscalingSets []string
	for _, f := range files {
		if f.IsDir() {
			if strings.Contains(f.Name(), "autoscaling") {
				log.Info().Msgf("Found autoscaling test set: %s", f.Name())
				autoscalingSets = append(autoscalingSets, f.Name())
				continue
			}
			log.Info().Msgf("Found basic test set: %s", f.Name())
			basicSets = append(basicSets, f.Name())
		}
	}

	// apply the test sets
	var errGroup errgroup.Group
	for _, path := range basicSets {
		func(path string, c pb.ContextBoxServiceClient) {
			errGroup.Go(func() error {
				log.Info().Msgf("Starting test set: %s", path)
				err := processTestSet(ctx, path, c, testLonghornDeployment)
				if err != nil {
					//in order to get errors from all goroutines in error group, print them here and just return simple error so test will fail
					log.Err(err).Msgf("Error in test sets %s ", path)
					return fmt.Errorf("error")
				}
				return nil
			})
		}(path, c)
	}
	if envs.Namespace != "" {
		for _, path := range autoscalingSets {
			func(path string, c pb.ContextBoxServiceClient) {
				errGroup.Go(func() error {
					log.Info().Msgf("Starting test set: %s", path)
					err := processTestSet(ctx, path, c,
						func(ctx context.Context, c *pb.Config) error {
							if err := testLonghornDeployment(ctx, c); err != nil {
								return err
							}
							return testAutoscaler(ctx, c)
						})
					if err != nil {
						//in order to get errors from all goroutines in error group, print them here and just return simple error so test will fail
						log.Err(err).Msgf("Error in test sets %s", path)
						return fmt.Errorf("error")
					}
					return nil
				})
			}(path, c)
		}
	} else if len(autoscalingSets) > 0 {
		log.Warn().Msgf("Autoscaling tests are ignored in local deployment")
	}

	if err = errGroup.Wait(); err != nil {
		return fmt.Errorf("one or more test sets returned with error")
	}
	return nil
}

// processTestSet function will apply test set sequentially to Claudie
func processTestSet(ctx context.Context, setName string, c pb.ContextBoxServiceClient, testFunc func(ctx context.Context, c *pb.Config) error) error {
	// Set errCleanUp to clean up the infra on failure
	var errCleanUp, errIgnore error
	idInfo := idInfo{id: "", idType: -1}
	pathToTestSet := filepath.Join(testDir, setName)
	log.Info().Msgf("Working on the test set %s", pathToTestSet)

	// Defer clean up function
	defer func() {
		if errCleanUp != nil {
			if autoCleanUpFlag == "TRUE" {
				log.Info().Msgf("Deleting infra even after error due to flag \"-auto-clean-up\" set to %v", autoCleanUpFlag)
				// delete manifest from DB to clean up the infra
				if err := cleanUp(setName, idInfo.id, c); err != nil {
					log.Err(err).Msgf("error while cleaning up the infra for test set %s", setName)
				}
			}
		}
	}()

	manifestFiles, errIgnore := os.ReadDir(pathToTestSet)
	if errIgnore != nil {
		return fmt.Errorf("error while trying to read test manifests in %s : %w", pathToTestSet, errIgnore)
	}

	for _, manifest := range manifestFiles {
		// Apply test set manifest
		if errIgnore = applyManifest(setName, pathToTestSet, manifest, &idInfo, c); errIgnore != nil {
			// https://github.com/berops/claudie/pull/243#issuecomment-1218237412
			if errors.Is(errIgnore, errHiddenOrDir) {
				continue
			}
			return fmt.Errorf("error applying test set %s, manifest %s from %s : %w", manifest.Name(), setName, manifest.Name(), errIgnore)
		}

		// Wait until test manifest has been processed
		if errCleanUp = configChecker(ctx, c, pathToTestSet, manifest.Name(), idInfo); errCleanUp != nil {
			if errors.Is(errCleanUp, errInterrupt) {
				log.Warn().Msgf("Testing-framework received interrupt signal, aborting test checking")
				// Do not return error, since it was an interrupt
				return nil
			}
			return fmt.Errorf("error while monitoring manifest %s from test set %s : %w", manifest.Name(), setName, errCleanUp)
		}

		// Run additional tests
		if testFunc != nil {
			// Get config from DB
			var res *pb.GetConfigFromDBResponse
			if res, errCleanUp = c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: idInfo.id, Type: idInfo.idType}); errCleanUp != nil {
				return fmt.Errorf("error while checking test for config %s from manifest %s, test set %s : %w", res.Config.Name, manifest.Name(), setName, errCleanUp)
			}

			// Start additional tests
			log.Info().Msgf("Starting additional tests for manifest %s from %s", manifest.Name(), setName)
			if errCleanUp = testFunc(ctx, res.Config); errCleanUp != nil {
				if errors.Is(errCleanUp, errInterrupt) {
					log.Warn().Msgf("Testing-framework received interrupt signal, aborting test checking")
					// Do not return error, since it was an interrupt
					return nil
				}
				return fmt.Errorf("error while performing additional test for manifest %s from %s : %w", manifest.Name(), setName, errCleanUp)
			}
		} else {
			log.Debug().Msgf("No additional tests, manifest %s from %s is done", manifest.Name(), setName)
		}
		log.Info().Msgf("Manifest %s from %s is done...", manifest.Name(), pathToTestSet)
	}

	// Clean up
	log.Info().Msgf("Deleting the infra from test set %s", setName)

	// Delete manifest from DB to clean up the infra as errCleanUp is nil and deferred function will not clean up.
	if errIgnore = cleanUp(setName, idInfo.id, c); errIgnore != nil {
		return fmt.Errorf("error while cleaning up the infra for test set %s : %w", setName, errIgnore)
	}
	return nil
}

func applyManifest(setName, pathToTestSet string, manifest fs.DirEntry, idInfo *idInfo, c pb.ContextBoxServiceClient) error {
	if manifest.IsDir() || manifest.Name()[0] == '.' {
		return errHiddenOrDir
	}

	// create a path and read the file
	manifestPath := filepath.Join(pathToTestSet, manifest.Name())
	yamlFile, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("error while reading the manifest %s : %w", manifestPath, err)
	}
	manifestName, err := getManifestName(yamlFile)
	if err != nil {
		return fmt.Errorf("error while getting the manifest name from %s : %w", manifestPath, err)
	}

	if envs.Namespace != "" {
		err = clusterTesting(yamlFile, setName, pathToTestSet, manifestName, c)
		idInfo.id = manifestName
		idInfo.idType = pb.IdType_NAME
		if err != nil {
			return fmt.Errorf("error while applying manifest %s : %w", manifest.Name(), err)
		}
	} else {
		idInfo.id, err = localTesting(yamlFile, manifestName, c)
		idInfo.idType = pb.IdType_HASH
		if err != nil {
			return fmt.Errorf("error while applying manifest %s : %w", manifest.Name(), err)
		}
	}
	return nil
}

// clusterTesting will perform actions needed for testing framework to function in k8s cluster deployment
// this option is only used when NAMESPACE env var has been found
// this option is testing the whole claudie
func clusterTesting(yamlFile []byte, setName, pathToTestSet, manifestName string, c pb.ContextBoxServiceClient) error {
	// get the id from manifest file
	id, err := getManifestName(yamlFile)
	idType := pb.IdType_NAME
	if err != nil {
		return fmt.Errorf("error while getting an id for %s : %w", manifestName, err)
	}

	if err = applySecret(yamlFile, pathToTestSet, setName); err != nil {
		return fmt.Errorf("error while applying a secret for %s : %w", setName, err)
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

// cleanUp will delete manifest from claudie which will trigger infra deletion
// it deletes a secret if claudie is deployed in k8s cluster
// it calls for a deletion from database directly if claudie is deployed locally
func cleanUp(setName, id string, c pb.ContextBoxServiceClient) error {
	if envs.Namespace != "" {
		// delete secret from namespace
		if err := deleteSecret(setName); err != nil {
			return fmt.Errorf("error while deleting the secret %s from %s : %w", id, envs.Namespace, err)
		}
	} else {
		// delete config from database
		if err := cbox.DeleteConfig(c, &pb.DeleteConfigRequest{Id: id, Type: pb.IdType_HASH}); err != nil {
			return fmt.Errorf("error while deleting the manifest from test set %s : %w", id, err)
		}
	}
	return nil
}
