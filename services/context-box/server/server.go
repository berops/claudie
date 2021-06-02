package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"
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

//TODO: Change byte to project structure
type configItem struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Manifest     string             `bson:"manifest"`
	DesiredState []byte             `bson:"desiredState"`
	CurrentState []byte             `bson:"currentState"`
	MsChecksum   []byte             `bson:"msChecksum"`
	DsChecksum   []byte             `bson:"dsChecksum"`
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
		MsChecksum:   data.MsChecksum,
		DsChecksum:   data.DsChecksum,
	}
}

func saveToDB(config *pb.Config) (*pb.Config, error) {
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
	data.MsChecksum = config.GetMsChecksum()
	data.DsChecksum = config.GetDsChecksum()

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

		return &pb.Config{}, nil
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
	return config, nil
}

func (*server) SaveConfigScheduler(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("SaveConfigScheduler request")
	config := req.GetConfig()

	// Get config from the DB
	var data configItem
	oid, err := primitive.ObjectIDFromHex(config.GetId()) //convert id to mongo type id (oid)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument,
			fmt.Sprintln("Cannot parse ID"),
		)
	}
	filter := bson.M{"_id": oid}
	if err := collection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		log.Fatalln(err)
	}

	// Compare manifest checksums
	fmt.Println("Config checksum:", config.MsChecksum)
	fmt.Println("Data checksum:", data.MsChecksum)
	if string(config.MsChecksum) != string(data.MsChecksum) {
		log.Println("Manifest checksums mismatch. Desired State will be not saved.")
		return &pb.SaveConfigResponse{Config: config}, nil
	}

	config.DsChecksum = config.MsChecksum

	config, err1 := saveToDB(config)
	if err1 != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err1),
		)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

func (*server) SaveConfigFrontEnd(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("SaveConfigFrontEnd request")
	config := req.GetConfig()
	msChecksum := md5.Sum([]byte(config.GetManifest())) //Calculate md5 hash for a manifest file
	config.MsChecksum = msChecksum[:]                   //Creating a slice using an array you can just make a simple slice expression

	config, err := saveToDB(config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfig is a gRPC service: function returns one config from the queue
func (*server) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfig request")
	if len(queue) > 0 {
		var config *configItem
		config, queue = queue[0], queue[1:]
		return &pb.GetConfigResponse{Config: dataToConfigPb(config)}, nil
	}
	return &pb.GetConfigResponse{}, nil
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	log.Println("GetAllConfigs request")
	var res []*pb.Config //slice of configs

	configs, err := getAllFromDB() //get all configs from database
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		res = append(res, dataToConfigPb(config)) //add them into protobuf in the right format
	}

	return &pb.GetAllConfigsResponse{Configs: res}, nil
}

// DeleteConfig is a gRPC service: function deletes one specified config from the DB and returns it's ID
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

//getAllFromDB gets all configs from the database and returns slice of *configItem
func getAllFromDB() ([]*configItem, error) {
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
	configs, err := getAllFromDB()
	if err != nil {
		return nil, err
	}

	for _, config := range configs {
		unique := true
		if string(config.DsChecksum) != string(config.MsChecksum) {
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
	client, err := mongo.NewClient(options.Client().ApplyURI(urls.DatabaseURL)) //client represents connection object do db
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	fmt.Println("MongoDB connected via", urls.DatabaseURL)
	collection = client.Database("platform").Collection("config")
	defer client.Disconnect(context.TODO()) //closing MongoDB connection

	// Start ContextBox Service
	lis, err := net.Listen("tcp", urls.ContextBoxURL)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("ContextBox service is listening on", urls.ContextBoxURL)

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
			log.Println("Queue content:", queue)
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
