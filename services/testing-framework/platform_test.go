package testingframework

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
	maxTimeout = 120 //checking each 30s, so max allowed time for operation to finish is maxTimeout * 30 [seconds]
)

//ClientConnection will return new client connection to Context-box
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

//applyTestSet function will apply test set sequantially to a platform
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

		id, err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
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
		go configChecker(done, c, id, file.Name())

		<-done //wait until test config has been processed
	}
	// delete the nodes
	log.Println("Deleting the clusters from test set:", pathToSet)
	err = cbox.DeleteConfig(c, id)
	if err != nil {
		return err
	}

	return nil
}

//configChecker function will check if the config has been applied every 30s
func configChecker(done chan struct{}, c pb.ContextBoxServiceClient, configID string, configName string) {
	var timeout int
	for {
		// if CSchecksum == DSchecksum, the config has been processed
		config, err := c.GetConfigById(context.Background(), &pb.GetConfigByIdRequest{
			Id: configID,
		})
		if err != nil {
			log.Fatal("Got error while waiting for config to finish:", err)
		}
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
		log.Printf("Waiting for %s to finish... [ %ds elapsed ]", configName, timeout*30)
	}
	done <- struct{}{} //send signal that config has been processed, unblock the applyTestSet
}

//equals function will check if two checksums are equal
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
