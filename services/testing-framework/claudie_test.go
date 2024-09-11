package testingframework

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
)

const testDir = "test-sets"

var (
	// get env var from runtime directly so we do not pollute original envs package by unnecessary variables
	autoCleanUpFlag = os.Getenv("AUTO_CLEAN_UP")
)

// TestClaudie will start all the test cases specified in tests directory
func TestClaudie(t *testing.T) {
	if testing.Short() {
		t.Skipf("skipping testing-framework test-case")
	}

	utils.InitLog("testing-framework")

	ctx, cancel := context.WithCancel(context.Background())
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM)
		defer stop()

		<-ctx.Done()
		cancel()

		err := ctx.Err()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err == nil {
			log.Info().Msgf("Received SIGTERM signal")
			err = errors.New("program interruption signal")
		}
		return err
	})

	// start E2E tests in separate goroutines
	group.Go(func() error {
		err := testClaudie(ctx)
		if err == nil {
			cancel()
		}
		return err
	})

	// wait for either test to finish or interrupt signal to occur
	// return either error from ctx, or error from testClaudie()
	if err := group.Wait(); err != nil {
		t.Error(err)
	}
}

func testClaudie(ctx context.Context) error {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return err
	}
	defer manager.Close()

	log.Info().Msg("---- Starting the tests ----")

	// loop through the directory and list files inside
	files, err := os.ReadDir(testDir)
	if err != nil {
		return fmt.Errorf("error while trying to read test sets: %w", err)
	}

	// save all the test set paths
	var basicSets, autoscalingSets []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		if strings.Contains(f.Name(), "autoscaling") {
			log.Info().Msgf("Found autoscaling test set: %s", f.Name())
			autoscalingSets = append(autoscalingSets, f.Name())
			continue
		}

		log.Info().Msgf("Found basic test set: %s", f.Name())
		basicSets = append(basicSets, f.Name())
	}

	group, ctx := errgroup.WithContext(ctx)

	for _, path := range basicSets {
		group.Go(func() error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			log.Info().Msgf("Starting test set: %s", path)
			err := processTestSet(ctx, path, manager, testLonghornDeployment)
			if err == nil {
				log.Info().Msgf("test set: %s finished", path)
				return nil
			}

			//in order to get errors from all goroutines in error group, print them here and just return simple error so test will fail
			log.Err(err).Msgf("Error in test sets %s ", path)
			return err
		})
	}

	if envs.Namespace != "" {
		for _, path := range autoscalingSets {
			group.Go(func() error {
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()

				log.Info().Msgf("Starting test set: %s", path)

				err := processTestSet(ctx, path, manager, func(ctx context.Context, c *spec.Config) error {
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

				log.Info().Msgf("test set: %s finished", path)
				return nil
			})
		}
	}

	return group.Wait()
}

// processTestSet function will apply test set sequentially to Claudie
func processTestSet(ctx context.Context, setName string, m managerclient.ClientAPI, testFunc func(ctx context.Context, c *spec.Config) error) error {
	// Set errCleanUp to clean up the infra on failure
	var errCleanUp, errIgnore error
	var manifestName string
	pathToTestSet := filepath.Join(testDir, setName)
	log.Info().Msgf("Working on the test set %s", pathToTestSet)

	// Defer clean up function
	defer func() {
		if errCleanUp != nil {
			if autoCleanUpFlag == "TRUE" {
				log.Info().Msgf("Deleting infra even after error due to flag \"-auto-clean-up\" set to %v", autoCleanUpFlag)
				// delete manifest from DB to clean up the infra
				if err := cleanUp(setName, manifestName, m); err != nil {
					log.Err(err).Msgf("error while cleaning up the infra for test set %s", setName)
				}
			}
		}
	}()

	dir, errIgnore := os.ReadDir(pathToTestSet)
	if errIgnore != nil {
		return fmt.Errorf("error while trying to read test manifests in %s : %w", pathToTestSet, errIgnore)
	}

	for _, entry := range dir {
		// https://github.com/berops/claudie/pull/243#issuecomment-1218237412
		if entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		manifestPath := filepath.Join(pathToTestSet, entry.Name())

		rawManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("error while reading manifest %s : %w", manifestPath, err)
		}

		manifestName, err = getInputManifestName(rawManifest)
		if err != nil {
			return fmt.Errorf("error while getting the manifest name from %s : %w", manifestPath, err)
		}

		if errIgnore = applyManifest(ctx, manifestName, manifestPath, rawManifest, m); errIgnore != nil {
			return fmt.Errorf("error applying test set %s, manifest %s from %s : %w", entry.Name(), manifestName, setName, errIgnore)
		}

		// Wait until test manifest has been processed
		if errCleanUp = waitForDoneOrError(ctx, m, testset{
			Config:   manifestName,
			Set:      pathToTestSet,
			Manifest: entry.Name(),
		}); errCleanUp != nil {
			if errors.Is(errCleanUp, errInterrupt) {
				log.Warn().Msgf("Testing-framework received interrupt signal, aborting test checking")
				// Do not return error, since it was an interrupt
				return nil
			}
			return fmt.Errorf("error while monitoring manifest %s from test set %s : %w", entry.Name(), setName, errCleanUp)
		}

		resp, err := m.GetConfig(ctx, &managerclient.GetConfigRequest{Name: manifestName})
		if err != nil {
			err := fmt.Errorf("failed to fetch config %q: %w", manifestName, err)
			errCleanUp = err
			return err
		}

		// assert that current and desired state match.
		for cluster, state := range resp.Config.Clusters {
			equal := proto.Equal(state.Current, state.Desired)
			if !equal {
				err := fmt.Errorf("cluster %q from config %q has current and desired state that diverge after all tasks have been build successfully", cluster, manifestName)
				errCleanUp = err
				return err
			}
		}

		// Run additional tests
		if testFunc != nil {
			var resp *managerclient.GetConfigResponse
			resp, errCleanUp = m.GetConfig(ctx, &managerclient.GetConfigRequest{Name: manifestName})
			if errCleanUp != nil {
				return fmt.Errorf("error while checking test for config %s from manifest %s, test set %s : %w", manifestName, entry.Name(), setName, errCleanUp)
			}

			// Start additional tests
			log.Info().Msgf("Starting additional tests for manifest %s from %s", entry.Name(), setName)
			if errCleanUp = testFunc(ctx, resp.Config); errCleanUp != nil {
				if errors.Is(errCleanUp, errInterrupt) {
					log.Warn().Msgf("Testing-framework received interrupt signal, aborting test checking")
					// Do not return error, since it was an interrupt
					return nil
				}
				return fmt.Errorf("error while performing additional test for manifest %s from %s : %w", entry.Name(), setName, errCleanUp)
			}
		} else {
			log.Debug().Msgf("No additional tests, manifest %s from %s is done", entry.Name(), setName)
		}
		log.Info().Msgf("Manifest %s from %s is done...", entry.Name(), pathToTestSet)
	}

	// Clean up
	log.Info().Msgf("Deleting the infra from test set %s", setName)

	// Delete manifest from DB to clean up the infra as errCleanUp is nil and deferred function will not clean up.
	if errIgnore = cleanUp(setName, manifestName, m); errIgnore != nil {
		return fmt.Errorf("error while cleaning up the infra for test set %s : %w", setName, errIgnore)
	}
	return nil
}

func applyManifest(ctx context.Context, manifest, path string, raw []byte, m managerclient.ClientAPI) error {
	if envs.Namespace != "" {
		return clusterTesting(ctx, raw, path, manifest, m)
	}

	// testing locally - NOT TESTING THE OPERATOR!
	err := m.UpsertManifest(ctx, &managerclient.UpsertManifestRequest{
		Name:     manifest,
		Manifest: &managerclient.Manifest{Raw: string(raw)},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert manifest: %w", err)
	}
	log.Info().Msgf("Manifest %s %s has been saved...", path, manifest)
	return nil
}

// clusterTesting will perform actions needed for testing framework to function in k8s cluster deployment
// this option is only used when NAMESPACE env var has been found
// this option is testing the whole claudie
func clusterTesting(ctx context.Context, yamlFile []byte, pathToTestSet, manifestName string, m managerclient.CrudAPI) error {
	if err := applyInputManifest(yamlFile, pathToTestSet); err != nil {
		return fmt.Errorf("error while applying a input manifest for %s : %w", manifestName, err)
	}

	log.Info().Msgf("InputManifest %s has been saved...", manifestName)

	if err := waitForPickup(ctx, manifestName, m); err != nil {
		return fmt.Errorf("error while checking if with manifest %s is saved : %w", manifestName, err)
	}
	log.Info().Msgf("Manifest %s has been saved...", manifestName)
	return nil
}

func waitForPickup(ctx context.Context, config string, m managerclient.CrudAPI) error {
	elapsed, tick := 0, 5

	ticker := time.NewTicker(time.Duration(tick) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return err
			}
			return errors.New("context done")
		case <-ticker.C:
			elapsed += tick
			log.Info().Msgf("Waiting for input manifest for config with id %s to be picked up by the operator... [ %ds elapsed...]", config, elapsed)

			if elapsed > maxTimeoutSave {
				return fmt.Errorf("input manifest for config %s has not been picked up by the operator in time, aborting", config)
			}

			resp, err := m.GetConfig(ctx, &managerclient.GetConfigRequest{Name: config})
			if err != nil {
				log.Warn().Msgf("failed to retrieve config %q, %v, trying again in  %vs", config, err, tick)
				break
			}
			if resp.Config.Manifest.State == spec.Manifest_Pending || resp.Config.Manifest.State == spec.Manifest_Scheduled {
				return nil
			}
		}
	}
}

// cleanUp will delete manifest from claudie which will trigger infra deletion
// it deletes an inputManifest if claudie is deployed in k8s cluster
// it calls for a deletion from database directly if claudie is deployed locally
func cleanUp(setName, name string, c managerclient.ManifestAPI) error {
	if envs.Namespace != "" {
		if err := deleteInputManifest(setName); err != nil {
			return fmt.Errorf("error while deleting the input manifest %s from %s : %w", name, envs.Namespace, err)
		}
	} else {
		// delete config from database
		if err := c.MarkForDeletion(context.Background(), &managerclient.MarkForDeletionRequest{Name: name}); err != nil {
			return fmt.Errorf("error while deleting the manifest from test set %s : %w", name, err)
		}
	}
	return nil
}
