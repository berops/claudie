package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/proto/pb"

	"google.golang.org/grpc/connectivity"
)

const defaultBuilderPort = 50051

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck() error {
	// Check if Builder can connect to Terraformer/Ansibler/Kube-eleven/Kuber/Context-box
	// Connection to these services are crucial for Builder, without them, the builder is NOT Ready
	services := map[string]string{
		"context-box": envs.ContextBoxURL,
		"terraformer": envs.TerraformerURL,
		"ansibler":    envs.AnsiblerURL,
		"kube-eleven": envs.KubeElevenURL,
		"kuber":       envs.KuberURL,
	}
	for service, url := range services {
		if cc, err := utils.GrpcDialWithRetryAndBackoff(service, url); err != nil {
			return err
		} else {
			if err := cc.Close(); err != nil {
				return fmt.Errorf("error closing connection for %s in health check function : %w", service, err)
			}
		}
	}
	return nil
}

func main() {
	utils.InitLog("builder")

	if err := run(); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

func run() error {
	conn, err := utils.GrpcDialWithRetryAndBackoff("context-box", envs.ContextBoxURL)
	if err != nil {
		return fmt.Errorf("failed to connect to context-box on %s : %w", envs.ContextBoxURL, err)
	}
	defer utils.CloseClientConnection(conn)

	log.Info().Msgf("Initiated connection Context-box: %s, waiting for connection to be in ready state", envs.ContextBoxURL)

	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultBuilderPort), healthCheck).StartProbes()

	group, ctx := errgroup.WithContext(context.Background())

	group.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received signal %v", sig)
			err = errors.New("interrupt signal")
		}

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	group.Go(func() error {
		client := pb.NewContextBoxServiceClient(conn)
		prevState := conn.GetState()
		group := sync.WaitGroup{}

		worker.NewWorker(
			ctx,
			5*time.Second,
			func() error {
				if conn.GetState() == connectivity.Ready {
					// Connection became ready.
					if prevState != connectivity.Ready {
						log.Info().Msgf("Connection to Context-box is now ready")
					}
					prevState = connectivity.Ready
				} else {
					log.Warn().Msgf("Connection to Context-box is not ready yet")
					log.Debug().Msgf("Connection to Context-box is %s, waiting for the service to be reachable", conn.GetState().String())

					prevState = conn.GetState()
					conn.Connect() // try connecting to the service.

					return nil
				}
				return configProcessor(client, &group)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Builder stopped checking for new configs")
		log.Info().Msgf("Waiting for already started configs to finish processing")

		group.Wait()
		log.Debug().Msgf("All spawned go-routines finished")

		return nil
	})

	return group.Wait()
}
