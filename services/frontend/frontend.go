package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/Berops/platform/internal/envs"
	"github.com/Berops/platform/internal/healthcheck"
	"github.com/Berops/platform/internal/manifest"
	"github.com/Berops/platform/internal/utils"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
)

const (
	defaultFrontendPort = 50058
	sleepDuration       = 60 * 5 // 5 minutes
)

var (
	manifestDir = os.Getenv("MANIFEST_DIR")
)

func ClientConnection(ctx context.Context) (pb.ContextBoxServiceClient, error) {
	cc, err := utils.GrpcDialWithInsecureAndBackoff(ctx, "context-box", envs.ContextBoxURL)
	if err != nil {
		return nil, err
	}
	log.Info().Msg("Connected to cbox")

	return pb.NewContextBoxServiceClient(cc), nil
}

func SaveFiles(c pb.ContextBoxServiceClient) error {
	if c == nil {
		return fmt.Errorf("nil client received")
	}

	// loop through the directory and list files inside
	files, err := ioutil.ReadDir(manifestDir)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test sets: %v", err)
		return err
	}

	// get all saved configs
	configsToDelete, err := cbox.GetAllConfigs(c)
	if err != nil {
		log.Fatal().Msgf("Failed to get all configs from the database")
		return err
	}

	log.Info().Msgf("Found %d files in %v", len(files), manifestDir)
	log.Info().Msgf("Found %d files in database", len(configsToDelete.Configs))

	for _, file := range files {
		// read file
		var manifest manifest.Manifest
		filePath := filepath.Join(manifestDir, file.Name())
		strManifest, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatal().Err(err)
			return err
		}
		// syntax check can be done here
		err = yaml.Unmarshal(strManifest, &manifest)
		if err != nil {
			log.Fatal().Msgf("Failed to parse manifest file, Err:%v", err)
			return err
		}

		// remove this config from configsToDelete
		configsToDelete.Configs, err = removeConfig(configsToDelete.Configs, manifest.Name)
		if err != nil {
			log.Info().Msgf("No config saved in the database")
		}
		_, err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     manifest.Name,
				Manifest: string(strManifest),
			},
		})
		if err != nil {
			log.Fatal().Msgf("Error while saving the config: %v err: %v", file.Name(), err)
			return err
		}
		log.Info().Msgf("File %s has been saved to the database", filePath)
	}

	for _, config := range configsToDelete.Configs {
		if err := cbox.DeleteConfig(c, config.Id, pb.IdType_HASH); err != nil {
			log.Error().Msgf("Failed to the delete %s with id %s : %v", config.Name, config.Id, err)
		}
	}
	log.Info().Msg("Saved all files")
	return nil
}

func removeConfig(configs []*pb.Config, configName string) ([]*pb.Config, error) {
	if len(configs) <= 0 {
		return configs, fmt.Errorf("no Config present")
	}
	var index = 0
	for i, config := range configs {
		if config.Name == configName {
			index = i
			break
		}
	}
	configs = append(configs[0:index], configs[index+1:]...)
	return configs, nil
}

func healthCheck() error {
	err := SaveFiles(nil)
	if err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func main() {
	utils.InitLog("frontend")

	// Initialize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultFrontendPort), healthCheck)
	healthChecker.StartProbes()

	// set reconnect timeout to 5 mins.
	ctx, cancel := context.WithTimeout(context.Background(), 5*60*time.Second)

	client, err := ClientConnection(ctx)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	cancel()

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch
		return errors.New("scheduler interrupt signal")
	})
	for {
		// list and upload manifest
		err := SaveFiles(client)
		if err != nil {
			panic(err)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}
