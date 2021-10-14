package testingframework

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testDir    = "tests"
	maxTimeout = 3600 // max allowed time for operation to finish in [seconds]
)

// ClientConnection will return new client connection to Context-box
func ClientConnection() pb.ContextBoxServiceClient {
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Msgf("could not connect to server: %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
}

// TestPlatform will start all the test cases specified in tests directory
func TestPlatform(t *testing.T) {
	var err error
	c := ClientConnection()
	log.Info().Msg("----Starting the tests----")

	// loop through the directory and list files inside
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test sets: %v", err)
	}

	// save all the test set paths
	var pathsToSets []string
	for _, f := range files {
		if f.IsDir() {
			log.Info().Msgf("Found test set: %s", f.Name())
			setFile := filepath.Join(testDir, f.Name())
			pathsToSets = append(pathsToSets, setFile)
		}
	}

	// apply test sets sequentially - while framework is still in dev
	for _, path := range pathsToSets {
		err = applyTestSet(path, c)
		if err != nil {
			log.Fatal().Msgf("Error while processing %s : %v", path, err)
			break
		}
	}

	require.NoError(t, err)
}

// applyTestSet function will apply test set sequantially to a platform
func applyTestSet(pathToSet string, c pb.ContextBoxServiceClient) error {
	done := make(chan struct{})
	var id string

	log.Info().Msgf("Working on the test set: %s", pathToSet)

	files, err := ioutil.ReadDir(pathToSet)
	if err != nil {
		log.Fatal().Msgf("Error while trying to read test configs: %v", err)
	}

	for _, file := range files {
		setFile := filepath.Join(pathToSet, file.Name())
		manifest, errR := ioutil.ReadFile(setFile)
		if errR != nil {
			log.Fatal().Err(errR)
		}

		id, err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     file.Name(),
				Id:       id,
				Manifest: string(manifest),
			},
		})

		if err != nil {
			log.Fatal().Msgf("Error while saving a config: %v", err)
			return err
		}
		go configChecker(done, c, id, file.Name())
		// wait until test config has been processed
		<-done
	}
	// delete the nodes
	log.Info().Msgf("Deleting the clusters from test set: %s", pathToSet)
	err = cbox.DeleteConfig(c, id)
	if err != nil {
		return err
	}

	return nil
}

// configChecker function will check if the config has been applied every 30s
func configChecker(done chan struct{}, c pb.ContextBoxServiceClient, configID string, configName string) {
	var counter int
	sleepSec := 30
	for {
		elapsedSec := counter * sleepSec
		// if CSchecksum == DSchecksum, the config has been processed
		config, err := c.GetConfigById(context.Background(), &pb.GetConfigByIdRequest{
			Id: configID,
		})
		if err != nil {
			log.Fatal().Msgf("Got error while waiting for config to finish: %v", err)
		}
		if config != nil {
			cs := config.Config.CsChecksum
			ds := config.Config.DsChecksum
			if checksumsEqual(cs, ds) {
				break
			}
		}
		if elapsedSec == maxTimeout {
			log.Fatal().Msgf("Test took too long... Aborting on timeout %d seconds", maxTimeout)
		}
		time.Sleep(time.Duration(sleepSec) * time.Second)
		counter++
		log.Info().Msgf("Waiting for %s to finish... [ %ds elapsed ]", configName, elapsedSec)
	}
	//send signal that config has been processed, unblock the applyTestSet
	done <- struct{}{}
}

// checksumsEq will check if two checksums are equal
func checksumsEqual(checksum1 []byte, checksum2 []byte) bool {
	if len(checksum1) > 0 && len(checksum2) > 0 && bytes.Compare(checksum1, checksum2) == 0 {
		return true
	} else {
		return false
	}
}
