package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var collection *mongo.Collection

var queue []*configItem

type server struct{}

type configItem struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Name          string             `bson:"name"`
	Manifest      string             `bson:"manifest"`
	DesiredState  []byte             `bson:"desiredState"`
	CurrentState  []byte             `bson:"currentState"`
	IsNewManifest bool               `bson:"isNewManifest"`
}

func dataToConfigPb(data *configItem) *pb.Config {
	var desiredState *pb.Project = new(pb.Project)
	err := proto.Unmarshal(data.DesiredState, desiredState)
	if err != nil {
		log.Fatalln("Error while Unmarshal desiredState", err)
	}
	var currentState *pb.Project = new(pb.Project)
	err = proto.Unmarshal(data.CurrentState, currentState)
	if err != nil {
		log.Fatalln("Error while Unmarshal currentState", err)
	}

	return &pb.Config{
		Id:           data.ID.Hex(),
		Name:         data.Name,
		Manifest:     data.Manifest,
		DesiredState: desiredState,
		CurrentState: currentState,
	}
}

func (*server) SaveConfig(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("Save config request")
	config := req.GetConfig()

	//Convert desiredState and currentState to byte[] because we want to save them to the database
	desiredStateByte, errDS := proto.Marshal(config.DesiredState)
	if errDS != nil {
		log.Fatalln("Error while converting from protobuf to byte", errDS)
	}
	currentStateByte, errCS := proto.Marshal(config.CurrentState)
	if errCS != nil {
		log.Fatalln("Error while converting from protobuf to byte", errCS)
	}

	//Parse data and map it to configItem struct
	data := &configItem{}
	data.Name = config.GetName()
	data.Manifest = config.GetManifest()
	data.DesiredState = desiredStateByte
	data.CurrentState = currentStateByte
	data.IsNewManifest = config.GetIsNewManifest()

	//Check if ID exists
	if config.GetId() != "" {
		//Get id from config
		oid, err := primitive.ObjectIDFromHex(config.GetId())
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument,
				fmt.Sprintln("Cannot parse ID"),
			)
		}
		filter := bson.M{"_id": oid}

		_, err = collection.ReplaceOne(context.Background(), filter, data)
		if err != nil {
			return nil, status.Errorf(
				codes.NotFound,
				fmt.Sprintf("Cannot update config with specified ID: %v", err),
			)
		}

		return &pb.SaveConfigResponse{Config: config}, nil
	}

	//Add data to the collection if OID doesn't exist
	res, err := collection.InsertOne(context.Background(), data)
	if err != nil {
		// Return error in protobuf
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintln("Cannot convert to OID"),
		)
	}
	data.ID = oid
	config.Id = oid.Hex()
	//Return config with new ID
	return &pb.SaveConfigResponse{Config: config}, nil
}

func (*server) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfig request")
	var config *configItem
	config, queue = queue[0], queue[1:]
	return &pb.GetConfigResponse{Config: dataToConfigPb(config)}, nil
}

func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	log.Println("GetAllConfigs request")
	var res []*pb.Config //slice of configs

	configs, err := getAllConfigs() //get all configs from database
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		res = append(res, dataToConfigPb(config)) //add them into protobuf in the right format
	}

	return &pb.GetAllConfigsResponse{Configs: res}, nil
}

func (*server) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Println("DeleteConfig request")

	oid, err := primitive.ObjectIDFromHex(req.GetId()) //convert id to mongo type id (oid)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintln("Cannot parse ID"),
		)
	}
	filter := bson.M{"_id": oid}                                   //create filter for searching in the database
	res, err := collection.DeleteOne(context.Background(), filter) //delete object from the database
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Cannot delete config in MongoDB: %v", err),
		)
	}

	if res.DeletedCount == 0 { //check if the object was really deleted
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("Cannot find blog with the specified ID: %v", err),
		)
	}

	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
}

func getAllConfigs() ([]*configItem, error) {
	var configs []*configItem
	cur, err := collection.Find(context.Background(), primitive.D{{}}) //primitive.D{{}} finds all records in the collection
	if err != nil {
		return nil, err
	}
	defer func() {
		err := cur.Close(context.Background())
		if err != nil {
			log.Fatalln(err)
		}
	}()
	for cur.Next(context.Background()) { //Iterate through cur and extract all data
		data := &configItem{}   //initialize empty struct
		err := cur.Decode(data) //Decode data from cursor to data
		if err != nil {         //check error
			return nil, err
		}
		configs = append(configs, data) //append decoded data (config) to res (response) slice
	}

	return configs, nil
}

func configCheck(queue []*configItem) ([]*configItem, error) {
	configs, err := getAllConfigs()
	if err != nil {
		return nil, err
	}

	// Check all configs for true bool in IsNewManifest add these configs to the queue if they are not there already
	for _, config := range configs {
		if config.IsNewManifest { //If IsNewManifest is true, check if it is already in the queue
			unique := true
			for _, item := range queue {
				if config.ID == item.ID {
					unique = false
					break
				}
			}
			if unique {
				queue = append(queue, config)
			}
		}
	}
	return queue, nil
}

func main() {

	// If code crash, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Connect to MongoDB
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017")) //client represents connection object do db
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	collection = client.Database("platform").Collection("config")
	defer client.Disconnect(context.TODO()) //closing MongoDB connection

	// Start ContextBox Service
	lis, err := net.Listen("tcp", ports.ContextBoxPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("ContextBox service is listening on", ports.ContextBoxPort)

	s := grpc.NewServer()
	pb.RegisterContextBoxServiceServer(s, &server{})

	go func() {
		fmt.Println("Starting Server...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		for {
			queue, err = configCheck(queue)
			if err != nil {
				log.Fatalln("Error while configCheck", err)
			}
			log.Println(queue)
			time.Sleep(10 * time.Second)
		}
	}()

	// Block until a signal is received
	<-ch
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("Closing the listener")
	lis.Close()
	fmt.Println("End of Program")
}
