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

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/services/scheduler/manifest"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
)

const (
	defaultFrontendPort = 50058
	manifestDir         = "/input-manifests"
	sleepDuration       = 60 * 15 // 15 minutes
)

func ClientConnection() pb.ContextBoxServiceClient {
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msg("Connected to cbox")

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
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

	log.Info().Msgf("Found %d files in %v", len(files), manifestDir)

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
		err = yaml.Unmarshal([]byte(strManifest), &manifest)
		if err != nil {
			log.Fatal().Err(err)
			return err
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
	}
	log.Info().Msg("Saved all files")
	return nil
}

func healthCheck() error {
	err := SaveFiles(nil)
	if err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func main() {
	utils.InitLog("frontend", "GOLANG_LOG")

	client := ClientConnection()

	// Initialize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultFrontendPort), healthCheck)
	healthChecker.StartProbes()

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
		time.Sleep(time.Duration(sleepDuration * time.Second))
	}

}
