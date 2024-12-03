package testingframework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"

	"google.golang.org/protobuf/proto"
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

	loggerutils.Init("testing-framework")

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

	// no test sets.
	if _, err := os.Stat(testDir); errors.Is(err, os.ErrNotExist) {
		log.Info().Msg("---- No tests found ----")
		return nil
	}

	// loop through the directory and list files inside
	files, err := os.ReadDir(testDir)
	if err != nil {
		return fmt.Errorf("error while trying to read test sets: %w", err)
	}

	// save all the test set paths
	var basicSets, autoscalingSets, succeedsOnLastSets []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		if strings.Contains(f.Name(), "autoscaling") {
			log.Info().Msgf("Found autoscaling test set: %s", f.Name())
			autoscalingSets = append(autoscalingSets, f.Name())
			continue
		}

		if strings.HasPrefix(f.Name(), "succeeds-on-last") {
			log.Info().Msgf("Found succeeds on last test set: %s", f.Name())
			succeedsOnLastSets = append(succeedsOnLastSets, f.Name())
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
			err, cleanup := processTestSet(ctx, path, false, manager, testLonghornDeployment)
			if err == nil || errors.Is(err, errInterrupt) || errors.Is(err, context.Canceled) {
				if err := cleanup(); err != nil {
					log.Err(err).Msgf("Error in cleaning up test set %s", path)
				}
				log.Info().Msgf("test set: %s finished", path)
				return nil
			}
			log.Err(err).Msgf("Error in test sets %s ", path)
			return err
		})
	}

	for _, path := range succeedsOnLastSets {
		group.Go(func() error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			log.Info().Msgf("Starting test set: %s", path)
			err, cleanup := processTestSet(ctx, path, true, manager, testLonghornDeployment)
			if err == nil || errors.Is(err, errInterrupt) || errors.Is(err, context.Canceled) {
				if err := cleanup(); err != nil {
					log.Err(err).Msgf("Error in cleaning up test set %s", path)
				}
				log.Info().Msgf("test set: %s finished", path)
				return nil
			}
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

				err, cleanup := processTestSet(
					ctx,
					path,
					false,
					manager,
					func(ctx context.Context, c *spec.Config) error {
						if err := testLonghornDeployment(ctx, c); err != nil {
							return err
						}
						return testAutoscaler(ctx, c)
					},
				)
				if err == nil || errors.Is(err, errInterrupt) || errors.Is(err, context.Canceled) {
					if err := cleanup(); err != nil {
						log.Err(err).Msgf("Error in cleaning up test set %s", path)
					}
					log.Info().Msgf("test set: %s finished", path)
					return nil
				}
				log.Err(err).Msgf("Error in test sets %s ", path)
				// failed to build, no cleanup(), keep infra for debugging.
				return err
			})
		}
	}

	return group.Wait()
}

// processTestSet function will apply test set sequentially to Claudie
func processTestSet(
	ctx context.Context,
	setName string,
	continueOnBuildError bool,
	m managerclient.ClientAPI,
	testFunc func(ctx context.Context, c *spec.Config) error,
) (error, func() error) {
	pathToTestSet := filepath.Join(testDir, setName)
	log.Info().Msgf("Working on the test set %s", pathToTestSet)

	var (
		manifestName string
		nocleanup    = func() error { return nil }
		cleanup      = func() error {
			if autoCleanUpFlag == "TRUE" {
				log.Info().Msgf("[%s] Deleting infra even after error due to flag \"-auto-clean-up\" set to %v", manifestName, autoCleanUpFlag)

				if err := cleanUp(setName, manifestName, m); err != nil {
					log.Err(err).Msgf("error while cleaning up the infra for test set %s", setName)
				}
			}
			return nil
		}
	)

	dir, err := os.ReadDir(pathToTestSet)
	if err != nil {
		return fmt.Errorf("error while trying to read test manifests in %s : %w", pathToTestSet, err), nocleanup
	}

	var configs []os.DirEntry
	for _, entry := range dir {
		// https://github.com/berops/claudie/pull/243#issuecomment-1218237412
		if entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		configs = append(configs, entry)
	}

	for i, entry := range configs {
		manifestPath := filepath.Join(pathToTestSet, entry.Name())

		rawManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("error while reading manifest %s : %w", manifestPath, err), nocleanup
		}

		manifestName, err = getInputManifestName(rawManifest)
		if err != nil {
			return fmt.Errorf("error while getting the manifest name from %s : %w", manifestPath, err), nocleanup
		}

		if err = applyManifest(manifestName, manifestPath, rawManifest, m); err != nil {
			return fmt.Errorf("error applying test set %s, manifest %s from %s : %w", entry.Name(), manifestName, setName, err), nocleanup
		}

		ts := testset{
			Config:   manifestName,
			Set:      pathToTestSet,
			Manifest: entry.Name(),
		}

		done, err := waitForDoneOrError(ctx, m, ts)
		if err != nil {
			if errors.Is(err, errInterrupt) {
				return err, cleanup
			}
			if i != len(configs)-1 && continueOnBuildError {
				continue
			}
			return fmt.Errorf("error while monitoring manifest %s from test set %s: %w", entry.Name(), setName, err), cleanup
		}

		for cluster, state := range done.Clusters {
			if !proto.Equal(state.Current, state.Desired) {
				err := fmt.Errorf("cluster %q from config %q has current and desired state that diverge after all tasks have been build successfully", cluster, manifestName)
				return err, cleanup
			}
		}

		log.Info().Msgf("Starting additional tests for manifest %s from %s", entry.Name(), setName)

		if err := testFunc(ctx, done); err != nil {
			return fmt.Errorf("error while performing additional test for manifest %s from %s : %w", entry.Name(), setName, err), cleanup
		}

		log.Info().Msgf("Manifest %s from %s is done...", entry.Name(), pathToTestSet)
	}

	// Clean up
	log.Info().Msgf("Deleting the infra from test set %s", setName)

	// Delete manifest from DB to clean up the infra as errCleanUp is nil and deferred function will not clean up.
	if err := cleanUp(setName, manifestName, m); err != nil {
		return fmt.Errorf("error while cleaning up the infra for test set %s : %w", setName, err), cleanup
	}
	return nil, nocleanup
}

func applyManifest(manifest, path string, raw []byte, m managerclient.ClientAPI) error {
	ctx := context.Background()

	if envs.Namespace != "" {
		if err := applyInputManifest(raw, path); err != nil {
			return fmt.Errorf("error while applying a input manifest for %s : %w", manifest, err)
		}

		log.Info().Msgf("InputManifest %s has been saved...", manifest)

		if err := waitForPickup(ctx, manifest, m); err != nil {
			return fmt.Errorf("error while checking if manifest %s is saved : %w", manifest, err)
		}
		log.Info().Msgf("Manifest %s has been saved...", manifest)
		return nil
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
