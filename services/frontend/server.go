package main

import (
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/services/scheduler/manifest"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
)

const (
	manifestDir   = "/input-manifest"
	sleepDuration = 60 * 15 // 15 minutes
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

func saveFiles(c pb.ContextBoxServiceClient) {
	// loop through the directory and list files inside
	files, err := ioutil.ReadDir(manifestDir)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test sets: %v", err)
	}

	log.Info().Msgf("Found %d files in %v", len(files), manifestDir)

	for _, file := range files {
		// read file
		var manifest manifest.Manifest
		filePath := filepath.Join(manifestDir, file.Name())
		strManifest, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatal().Err(err)
		}
		// syntax check can be done here
		err = yaml.Unmarshal([]byte(strManifest), &manifest)
		if err != nil {
			log.Fatal().Err(err)
		}

		_, err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     manifest.Name,
				Manifest: string(strManifest),
			},
		})
		if err != nil {
			log.Fatal().Msgf("Error while saving the config: %v err: %v", file.Name(), err)
		}
	}
	log.Info().Msg("Saved all files")
}

func main() {
	utils.InitLog("frontend", "GOLANG_LOG")

	client := ClientConnection()

	for {
		// list and upload manifest
		saveFiles(client)
		time.Sleep(time.Duration(sleepDuration * time.Second))
	}

}
