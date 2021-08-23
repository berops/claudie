package testing_framework

import (
	"context"
	"fmt"
	"time"

	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"google.golang.org/grpc"

	"io/ioutil"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testDir    = "tests"
	maxTimeout = 60
)

func ClientConnection() pb.ContextBoxServiceClient {
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
}

//TestPlatform will start all the test cases specified in tests directory
func TestPlatform(t *testing.T) {
	var err error
	c := ClientConnection()
	log.Println("----Starting the tests----")

	//loop through the directory and list files inside
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		log.Println("Error while trying to read test sets:", err)
	}

	//save all the test set paths
	var pathsToSets []string
	for _, f := range files {
		if f.IsDir() {
			pathsToSets = append(pathsToSets, testDir+"/"+f.Name())
			fmt.Println("Found test set:", f.Name())
		}
	}

	//apply test sets sequentially - while framework is still in dev
	for _, path := range pathsToSets {
		err = applyTestSet(path, c)
		if err != nil {
			log.Println("Error while processing", path, ":", err)
			break
		}
	}

	require.NoError(t, err)
}
func applyTestSet(pathToSet string, c pb.ContextBoxServiceClient) error {
	done := make(chan struct{})
	var id string

	log.Println("Working on the test set:", pathToSet)

	files, err := ioutil.ReadDir(pathToSet)
	if err != nil {
		log.Println("Error while trying to read test configs:", err)
	}

	for _, file := range files {
		manifest, errR := ioutil.ReadFile(pathToSet + "/" + file.Name())
		if errR != nil {
			log.Fatalln(errR)
		}

		id, err := cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     file.Name(),
				Id:       id,
				Manifest: string(manifest),
			},
		})

		if err != nil {
			log.Println("Error while saving a config")
			return err
		}
		go configChecker(done, c, id)

		<-done //wait until test config has been processed
	}
	if err != nil {
		return err
	}
	// delete the nodes
	_, err = c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id})

	if err != nil {
		return err
	}
	return nil
}

func configChecker(done chan struct{}, c pb.ContextBoxServiceClient, configId string) {
	var timeout byte
	for {
		// if CSchecksum == DSchecksum, the config has been processed
		config, err := c.GetConfigById(context.Background(), &pb.GetConfigByIdRequest{
			Id: configId,
		})
		if err != nil {
			log.Fatal("Got error while waiting for config to finish:", err)
		}
		log.Println(config.Config.DsChecksum, config.Config.CsChecksum)
		if config != nil {

			if equals(config.Config.DsChecksum, config.Config.CsChecksum) {
				break
			}
		}
		if timeout == maxTimeout {
			log.Fatal("Test took too long... Aborting")
		}
		time.Sleep(30 * time.Second)
		timeout++
	}
	done <- struct{}{} //send signal that config has been processed, unblock the applyTestSet
}

func equals(checksum []byte, checksum2 []byte) bool {
	if checksum == nil || checksum2 == nil {
		return false
	}
	for i := 0; i < len(checksum); i++ {
		if checksum[i] != checksum2[i] {
			return false
		}
	}
	return true
}
