package testingframework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
)

const (
	maxTimeout     = 8000    // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 30      // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

var (
	errInterrupt = errors.New("interrupt")
)

type testset struct{ Config, Set, Manifest string }

func waitForDoneOrError(ctx context.Context, manager managerclient.CrudAPI, set testset) error {
	elapsed := 0
	ticker := time.NewTicker(sleepSec * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errInterrupt
		case <-ticker.C:
			elapsed += sleepSec
			log.Info().Msgf("Waiting for %s from %s to finish... [ %ds elapsed ]", set.Manifest, set.Set, elapsed)
			if elapsed >= maxTimeout {
				return fmt.Errorf("test took too long... Aborting after %d seconds", maxTimeout)
			}

			res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: set.Config})
			if err != nil {
				return fmt.Errorf("error while waiting for config to finish: %w", err)
			}

			// Rolling update can have multiple stages, thus we also check for the manifest checksum equality.
			if res.Config.Manifest.State == spec.Manifest_Done {
				if bytes.Equal(res.Config.Manifest.LastAppliedChecksum, res.Config.Manifest.Checksum) {
					return nil
				}

				for c, s := range res.Config.Clusters {
					equal := proto.Equal(s.Current, s.Desired)
					if !equal {
						return fmt.Errorf("cluster %q has current state diverging from the desired state", c)
					}
				}
			}

			if res.Config.Manifest.State == spec.Manifest_Error {
				var err error

				for cluster, state := range res.Config.Clusters {
					if state.State.Status == spec.Workflow_ERROR {
						err = errors.Join(err, fmt.Errorf("----\nerror in cluster %s\n----\nStage: %s \n State: %s\n Description: %s", cluster, state.State.Stage, state.State.Status, state.State.Description))
					}
				}

				return err
			}
		}
	}
}

func getAutoscaledClusters(c *spec.Config) []*spec.K8Scluster {
	clusters := make([]*spec.K8Scluster, 0, len(c.Clusters))

	for _, s := range c.Clusters {
		if utils.IsAutoscaled(s.Current.GetK8S()) {
			clusters = append(clusters, s.Current.GetK8S())
		}
	}

	return clusters
}
