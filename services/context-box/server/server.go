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

	"github.com/Berops/platform/services/context-box/healthcheck"
	terraformer "github.com/Berops/platform/services/terraformer/client"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var collection *mongo.Collection

var queueScheduler []*configItem
var queueBuilder []*configItem
var tmpBuidler map[string]*configItem = make(map[string]*configItem)

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
	CsChecksum   []byte             `bson:"csChecksum"`
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
	data.CsChecksum = config.GetCsChecksum()

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

func getFromDB(id string) (configItem, error) {
	var data configItem
	oid, err := primitive.ObjectIDFromHex(id) //convert id to mongo type id (oid)
	if err != nil {
		log.Fatalln(err)
	}

	filter := bson.M{"_id": oid}
	if err := collection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		log.Fatalln("Error while finding ID in the DB", err)
	}
	return data, nil
}

func compareChecksums(ch1 string, ch2 string) bool {
	if ch1 != ch2 {
		log.Println("Manifest checksums mismatch. Nothing will be not saved.")
		return false
	}
	return true
}

func configCheck() error {
	configs, err := getAllFromDB()
	if err != nil {
		return err
	}

	for _, config := range configs {
		uniqueS := true
		uniqueB := true
		// Checking for Scheduler
		fmt.Println("")
		if string(config.DsChecksum) != string(config.MsChecksum) {
			for _, item := range queueScheduler {
				if config.ID == item.ID {
					uniqueS = false
					break
				}
			}
			if uniqueS {
				queueScheduler = append(queueScheduler, config)
			}
		}
		// Checking for Builder
		_, ok := tmpBuidler[config.Name]
		if (string(config.DsChecksum) != string(config.CsChecksum)) && !(ok) {
			for _, item := range queueBuilder {
				if config.ID == item.ID {
					uniqueB = false
					break
				}
			}
			if uniqueB {
				queueBuilder = append(queueBuilder, config)
			}
		}
	}

	return nil
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

func (*server) SaveConfigScheduler(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("SaveConfigScheduler request")
	config := req.GetConfig()

	// Get config with the same ID from the DB
	data, err := getFromDB(config.GetId())
	if err != nil {
		return nil, err
	}
	if !compareChecksums(string(config.MsChecksum), string(data.MsChecksum)) {
		return nil, nil
	}

	// Save new config to the DB
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

func (*server) SaveConfigBuilder(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("SaveConfigBuilder request")
	config := req.GetConfig()

	// Get config with the same ID from the DB
	data, err := getFromDB(config.GetId())
	if err != nil {
		return nil, err
	}
	if !compareChecksums(string(config.MsChecksum), string(data.MsChecksum)) {
		return nil, nil
	}

	// Save new config to the DB
	config.CsChecksum = config.DsChecksum
	config, err1 := saveToDB(config)
	if err1 != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err1),
		)
	}
	delete(tmpBuidler, config.Name)
	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfigScheduler is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfigScheduler request")
	if len(queueScheduler) > 0 {
		var config *configItem
		config, queueScheduler = queueScheduler[0], queueScheduler[1:] // This is like push from a queue
		return &pb.GetConfigResponse{Config: dataToConfigPb(config)}, nil
	}
	return &pb.GetConfigResponse{}, nil
}

// GetConfigBuilder is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfigBuilder request")
	if len(queueBuilder) > 0 {
		var config *configItem
		config, queueBuilder = queueBuilder[0], queueBuilder[1:] // This is like push from a queue
		tmpBuidler[config.Name] = config
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

	config, err := getFromDB(req.Id)
	if err != nil {
		log.Fatalln(err)
	}
	destroyConfigTerraformer(dataToConfigPb(&config)) //destroy infrastructure with terraformer

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

// destroyConfigTerraformer calls terraformer's DestroyInfrastructure function
func destroyConfigTerraformer(config *pb.Config) *pb.Config {
	// Create connection to Terraformer
	cc, err := grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}

	return res.GetConfig()
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

	// Set the context-box port
	contextboxPort := os.Getenv("CONTEXT_BOX_PORT")
	if contextboxPort == "" {
		contextboxPort = "50055" // Default value
	}

	// Start ContextBox Service
	lis, err := net.Listen("tcp", "0.0.0.0:"+contextboxPort)
	if err != nil {
		log.Fatalln("Failed to listen on", err)
	}
	fmt.Println("ContextBox service is listening on:", "0.0.0.0:"+contextboxPort)

	s := grpc.NewServer()
	pb.RegisterContextBoxServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewHealthChecker()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		for {
			err = configCheck()
			if err != nil {
				log.Fatalln("Error while configCheck", err)
			}
			log.Println("QueueScheduler content:", queueScheduler)
			log.Println("QueueBuilder content:", queueBuilder)
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
