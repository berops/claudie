package cbox

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
)

// func main() {
// 	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
// 	if err != nil {
// 		log.Fatalf("could not connect to server: %v", err)
// 	}
// 	defer cc.Close()

// 	// Creating the client
// 	c := pb.NewContextBoxServiceClient(cc)

// 	//Only for testing
// 	manifest, errR := ioutil.ReadFile("/Users/samuelstolicny/Github/Berops/platform/services/context-box/client/manifest.yaml")
// 	if errR != nil {
// 		log.Fatalln(errR)
// 	}
// 	config := &pb.Config{
// 		//Id:       "6049d7afc57394c1278f10a4",
// 		Name:     "test_without_states",
// 		Manifest: string(manifest),
// 		DesiredState: &pb.Project{Name: "test"},
// 		CurrentState: &pb.Project{Name: "test"},
// 	}

// 	SaveConfig(c, config)
// 	//GetConfig(c)
// 	//DeleteConfig(c)
// }

// SaveConfig calls Content-box gRPC server and saves configuration to the mongoDB database
// A new config file with Id will be created if ID is empty
func SaveConfig(c pb.ContextBoxServiceClient, config *pb.Config) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfig(context.Background(), &pb.SaveConfigRequest{Config: config})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

// GetConfig calls Content-box gRPC server and returns all configs from the mongoDB database
func GetConfig(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfig(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	return res, nil
}

// DeleteConfig deletes object from the mongoDB database with a specified Id
func DeleteConfig(c pb.ContextBoxServiceClient, id string) error {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id})
	if err != nil {
		log.Fatalf("Error happened while deleting: %v\n", err)
	}
	fmt.Println("Config was deleted", res)
	return nil
}
