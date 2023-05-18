package testingframework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const (
	maxTimeout     = 8000    // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 30      // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

var (
	// interrupt error
	errInterrupt = errors.New("interrupt")
	// hidden file error
	errHiddenOrDir = errors.New("hidden file")
)

type idInfo struct {
	id     string
	idType pb.IdType
}

// clientConnection will return new client connection to Context-box
func clientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create client connection to context-box")
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c, cc
}

// checksumsEqual will check if two checksums are equal
func checksumsEqual(checksum1 []byte, checksum2 []byte) bool {
	if len(checksum1) > 0 && len(checksum2) > 0 && bytes.Equal(checksum1, checksum2) {
		return true
	}
	return false
}

// configChecker function will check if the config has been applied every 30s
// it returns an interruptError if the pod/process is being terminated
func configChecker(ctx context.Context, c pb.ContextBoxServiceClient, testSetName, manifestName string, idInfo idInfo) error {
	counter := 1
	for {
		select {
		case <-ctx.Done():
			return errInterrupt
		default:
			elapsedSec := counter * sleepSec
			res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{
				Id:   idInfo.id,
				Type: idInfo.idType,
			})
			if err != nil {
				return fmt.Errorf("error while waiting for config to finish: %w", err)
			}
			if res.Config != nil {
				if err := getError(res.Config); err != nil {
					return fmt.Errorf("error while checking config %s : %w", res.Config.Name, err)
				}

				// if checksums are equal, the config has been processed by claudie
				if checksumsEqual(res.Config.MsChecksum, res.Config.CsChecksum) && checksumsEqual(res.Config.CsChecksum, res.Config.DsChecksum) {
					// manifest is done
					return nil
				}
			}
			if elapsedSec >= maxTimeout {
				return fmt.Errorf("test took too long... Aborting after %d seconds", maxTimeout)
			}
			time.Sleep(time.Duration(sleepSec) * time.Second)
			counter++
			log.Info().Msgf("Waiting for %s from %s to finish... [ %ds elapsed ]", manifestName, testSetName, elapsedSec)
		}
	}
}

func getAutoscaledClusters(c *pb.Config) []*pb.K8Scluster {
	clusters := make([]*pb.K8Scluster, 0, len(c.CurrentState.Clusters))
	for _, c := range c.CurrentState.Clusters {
		if utils.IsAutoscaled(c) {
			clusters = append(clusters, c)
		}
	}
	return clusters
}

func getError(c *pb.Config) error {
	var err error
	for cluster, state := range c.State {
		if state.Status == pb.Workflow_ERROR {
			err1 := fmt.Errorf("----\nerror in cluster %s\n----\nStage: %s \n State: %s\n Description: %s", cluster, state.Stage, state.Status, state.Description)
			if err == nil {
				err = err1
			} else {
				err = fmt.Errorf("%w \n %w", err1, err)
			}
		}
	}
	return err
}
