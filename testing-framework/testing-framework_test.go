package testing_framework

import (
	"fmt"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"google.golang.org/grpc"

	//"github.com/Berops/platform/proto/pb"
	//cbox "github.com/Berops/platform/services/context-box/client"
	//"github.com/Berops/platform/urls"
	"github.com/stretchr/testify/require"
	//"google.golang.org/grpc"
	"io/ioutil"
	"log"
	"testing"
)

const (
	testDir = "tests"
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

//TestPlatform will start all the test cases specified in platform/tests/
// should be run from root directory of this project like `go test testing-framework/testing-framework_test.go -run TestPlatform`
func TestPlatform(t *testing.T) {
	var err error
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
	/*for _, path := range pathsToSets {
		err = applyTestSet(path)
		if err != nil {
			log.Println("Error while processing", path, ":", err)
			break
		}
	}*/
	require.NoError(t, err)
}
func applyTestSet(pathToSet string) error {
	c := ClientConnection()
	var done chan struct{}

	log.Println("Working on the test set:", pathToSet)

	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		log.Println("Error while trying to read test configs:", err)
	}

	for _, path := range files {
		manifest, errR := ioutil.ReadFile(pathToSet + "/" + path.Name())
		if errR != nil {
			log.Fatalln(errR)
		}

		err = cbox.SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
			Config: &pb.Config{
				Name:     path.Name(),
				Manifest: string(manifest),
			},
		})

		//go configChecker(done,c , )

		<-done //wait until test config has been processed
	}

	if err != nil {
		return err
	}
	return nil
}

func configChecker(done chan struct{}, c pb.ContextBoxServiceClient, configId string) {
	for {
		// if CSchecksum == DSchecksum, the config has been processed

		break
	}
	done<-struct{}{}	//send signal that config has been processed, unblock the applyTestSet
}
