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

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/internal/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/claudie/proto/pb"
)

const defaultBuilderPort = 50051

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck() error {
	//Check if Builder can connect to Terraformer/Ansibler/Kube-eleven/Kuber
	//Connection to these services are crucial for Builder, without them, the builder is NOT Ready
	if cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL); err != nil {
		return err
	} else {
		if err := cc.Close(); err != nil {
			return fmt.Errorf("error closing connection in health check function : %w", err)
		}
	}
	if cc, err := utils.GrpcDialWithInsecure("ansibler", envs.AnsiblerURL); err != nil {
		return err
	} else {
		if err := cc.Close(); err != nil {
			return fmt.Errorf("error closing connection in health check function : %w", err)
		}
	}
	if cc, err := utils.GrpcDialWithInsecure("kubeEleven", envs.KubeElevenURL); err != nil {
		return err
	} else {
		if err := cc.Close(); err != nil {
			return fmt.Errorf("error closing connection in health check function : %w", err)
		}
	}
	if cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL); err != nil {
		return err
	} else {
		if err := cc.Close(); err != nil {
			return fmt.Errorf("error closing connection in health check function : %w", err)
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
	conn, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		return fmt.Errorf("failed to connect to context-box on %s : %w", envs.ContextBoxURL, err)
	}
	defer utils.CloseClientConnection(conn)

	log.Info().Msgf("Connected to Context-box: %s", envs.ContextBoxURL)

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
			err = errors.New("builder interrupt signal")
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
		group := sync.WaitGroup{}

		worker.NewWorker(
			ctx,
			5*time.Second,
			func() error {
				return configProcessor(client, &group)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Exited worker loop and stopped checking for new configs")
		log.Info().Msgf("Waiting for spawned go-routines to finish processing their work")

		group.Wait()

		log.Info().Msgf("All spawned go-routines finished")

		return nil
	})

	return group.Wait()
}
