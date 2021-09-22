package main

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	terraformer "github.com/Berops/platform/services/terraformer/client"
	"github.com/Berops/platform/worker"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/platform/healthcheck"
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

type server struct{}

type queue struct {
	configs []*configItem
}

const (
	defaultBuilderTTL   = 360
	defaultSchedulerTTL = 5
)

var (
	collection     *mongo.Collection
	queueScheduler queue
	queueBuilder   queue
	mutexDBsave    sync.Mutex
)

type configItem struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Manifest     string             `bson:"manifest"`
	DesiredState []byte             `bson:"desiredState"`
	CurrentState []byte             `bson:"currentState"`
	MsChecksum   []byte             `bson:"msChecksum"`
	DsChecksum   []byte             `bson:"dsChecksum"`
	CsChecksum   []byte             `bson:"csChecksum"`
	BuilderTTL   int                `bson:"BuilderTTL"`
	SchedulerTTL int                `bson:"SchedulerTTL"`
}

func (q *queue) contains(item *configItem) bool {
	if len(q.configs) == 0 {
		return false
	}
	for _, config := range q.configs {
		if config.Name == item.Name {
			return true
		}
	}
	return false
}

func (q *queue) push() (item *configItem, newQueue queue) {
	if len(q.configs) == 0 {
		return nil, *q
	}
	return q.configs[0], queue{
		configs: q.configs[1:],
	}
}

func dataToConfigPb(data *configItem) (*pb.Config, error) {
	var desiredState *pb.Project = new(pb.Project)
	err := proto.Unmarshal(data.DesiredState, desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while Unmarshal desiredState: %v", err)
	}

	var currentState *pb.Project = new(pb.Project)
	err = proto.Unmarshal(data.CurrentState, currentState)
	if err != nil {
		return nil, fmt.Errorf("error while Unmarshal currentState: %v", err)
	}

	return &pb.Config{
		Id:           data.ID.Hex(),
		Name:         data.Name,
		Manifest:     data.Manifest,
		DesiredState: desiredState,
		CurrentState: currentState,
		MsChecksum:   data.MsChecksum,
		DsChecksum:   data.DsChecksum,
		CsChecksum:   data.CsChecksum,
		BuilderTTL:   int32(data.BuilderTTL),
		SchedulerTTL: int32(data.SchedulerTTL),
	}, nil
}

func saveToDB(config *pb.Config) (*pb.Config, error) {
	//Convert desiredState and currentState to byte[] because we want to save them to the database
	desiredStateByte, errDS := proto.Marshal(config.DesiredState)
	if errDS != nil {
		return nil, fmt.Errorf("error while converting from protobuf to byte: %v", errDS)
	}

	currentStateByte, errCS := proto.Marshal(config.CurrentState)
	if errCS != nil {
		return nil, fmt.Errorf("error while converting from protobuf to byte: %v", errCS)
	}

	//Parse data and map it to the configItem struct
	data := &configItem{}
	data.Name = config.GetName()
	data.Manifest = config.GetManifest()
	data.DesiredState = desiredStateByte
	data.CurrentState = currentStateByte
	data.MsChecksum = config.GetMsChecksum()
	data.DsChecksum = config.GetDsChecksum()
	data.CsChecksum = config.GetCsChecksum()
	data.BuilderTTL = int(config.GetBuilderTTL())
	data.SchedulerTTL = int(config.GetSchedulerTTL())

	//Check if ID exists
	//If config has already some ID:
	if config.GetId() != "" {
		//Get id from config as oid
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
	} else {
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
	}
	return config, nil
}

func getFromDB(id string) (configItem, error) {
	var data configItem
	oid, err := primitive.ObjectIDFromHex(id) //convert id to mongo type id (oid)
	if err != nil {
		return data, err
	}

	filter := bson.M{"_id": oid}
	if err := collection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		return data, fmt.Errorf("error while finding ID in the DB: %v", err)
	}
	return data, nil
}

func compareChecksums(ch1 string, ch2 string) bool {
	if ch1 != ch2 {
		log.Println("Manifest checksums mismatch. Nothing will be saved.")
		return false
	}
	return true
}

func configCheck() error {
	mutexDBsave.Lock()
	configs, err := getAllFromDB()
	if err != nil {
		return err
	}
	// loop through config
	for _, config := range configs {
		// check if item is already in some queue
		if queueBuilder.contains(config) || queueScheduler.contains(config) {
			// some queue already has this particular config
			continue
		}

		// check for Scheduler
		if string(config.DsChecksum) != string(config.MsChecksum) {
			if config.SchedulerTTL <= 0 {
				config.SchedulerTTL = defaultSchedulerTTL

				c, err := dataToConfigPb(config)
				if err != nil {
					return err
				}

				if _, err := saveToDB(c); err != nil {
					return err
				}
				queueScheduler.configs = append(queueScheduler.configs, config)
				continue
			} else {
				config.SchedulerTTL = config.SchedulerTTL - 1
			}
		}

		// check for Builder
		if string(config.DsChecksum) != string(config.CsChecksum) {
			if config.BuilderTTL <= 0 {
				config.BuilderTTL = defaultBuilderTTL

				c, err := dataToConfigPb(config)
				if err != nil {
					return err
				}

				if _, err := saveToDB(c); err != nil {
					return err
				}
				queueBuilder.configs = append(queueBuilder.configs, config)
				continue

			} else {
				config.BuilderTTL = config.BuilderTTL - 1
			}
		}

		// save data if both TTL were substracted
		c, err := dataToConfigPb(config)
		if err != nil {
			return err
		}

		if _, err := saveToDB(c); err != nil {
			return nil
		}
	}
	mutexDBsave.Unlock()
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
	log.Println("CLIENT REQUEST: SaveConfigScheduler")
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
	config.SchedulerTTL = defaultSchedulerTTL
	mutexDBsave.Lock()
	config, err1 := saveToDB(config)
	mutexDBsave.Unlock()
	if err1 != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err1),
		)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

func (*server) SaveConfigFrontEnd(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("CLIENT REQUEST: SaveConfigFrontEnd")
	newConfig := req.GetConfig()
	msChecksum := md5.Sum([]byte(newConfig.GetManifest())) //Calculate md5 hash for a manifest file
	newConfig.MsChecksum = msChecksum[:]                   //Creating a slice using an array you can just make a simple slice expression

	if newConfig.GetId() != "" {
		//Check if there is already ID in the DB
		oldConfig, err := getFromDB(newConfig.GetId())
		if err != nil {
			log.Fatalln("Error while getting old newConfig from the DB", err)
		}
		oldConfigPb, err := dataToConfigPb(&oldConfig)
		if err != nil {
			log.Fatalln("Error while converting data to pb", err)
		}
		newConfig.CurrentState = oldConfigPb.CurrentState
	}

	newConfig, err := saveToDB(newConfig)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	return &pb.SaveConfigResponse{Config: newConfig}, nil
}

func (*server) SaveConfigBuilder(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("CLIENT REQUEST: SaveConfigBuilder")
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
	config.BuilderTTL = defaultBuilderTTL
	mutexDBsave.Lock()
	config, err1 := saveToDB(config)
	mutexDBsave.Unlock()
	if err1 != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err1),
		)
	}
	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfigById is a gRPC service: function returns one config from the DB
func (*server) GetConfigById(ctx context.Context, req *pb.GetConfigByIdRequest) (*pb.GetConfigByIdResponse, error) {
	log.Println("CLIENT REQUEST: GetConfigById")
	d, err := getFromDB(req.Id)
	if err != nil {
		return nil, err
	}
	config, err := dataToConfigPb(&d)
	if err != nil {
		return nil, err
	}
	return &pb.GetConfigByIdResponse{Config: config}, nil
}

// GetConfigScheduler is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfigScheduler request")
	if len(queueScheduler.configs) > 0 {
		var config *configItem
		config, queueScheduler = queueScheduler.push()
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, err
		}

		return &pb.GetConfigResponse{Config: c}, nil
	}
	return &pb.GetConfigResponse{}, nil
}

// GetConfigBuilder is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Println("GetConfigBuilder request")
	if len(queueBuilder.configs) > 0 {
		var config *configItem
		config, queueBuilder = queueBuilder.push()
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, err
		}

		return &pb.GetConfigResponse{Config: c}, nil
	}
	return &pb.GetConfigResponse{}, nil
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	log.Println("CLIENT REQUEST: GetAllConfigs")
	var res []*pb.Config //slice of configs

	configs, err := getAllFromDB() //get all configs from database
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, err
		}

		res = append(res, c) //add them into protobuf in the right format
	}

	return &pb.GetAllConfigsResponse{Configs: res}, nil
}

// DeleteConfig is a gRPC service: function deletes one specified config from the DB and returns it's ID
func (*server) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Println("CLIENT REQUEST: DeleteConfig")

	config, err := getFromDB(req.Id)
	if err != nil {
		return nil, err
	}

	c, err := dataToConfigPb(&config)
	if err != nil {
		return nil, err
	}

	_, err = destroyConfigTerraformer(c)
	if err != nil {
		return nil, err
	} //destroy infrastructure with terraformer

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
func destroyConfigTerraformer(config *pb.Config) (*pb.Config, error) {
	// Trim "tcp://" substring from urls.TerraformerURL
	trimmedTerraformerURL := strings.ReplaceAll(urls.TerraformerURL, ":tcp://", "")
	log.Println("Dial Terraformer: ", trimmedTerraformerURL)
	// Create connection to Terraformer
	cc, err := grpc.Dial(trimmedTerraformerURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to server: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

func configChecker() error {
	if err := configCheck(); err != nil {
		return fmt.Errorf("error while configCheck: %v", err)
	}

	log.Println("QueueScheduler content:", queueScheduler)
	log.Println("QueueBuilder content:", queueBuilder)

	return nil
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
	healthService := healthcheck.NewServerHealthChecker("50055", "CONTEXT_BOX_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(10*time.Second, ctx, configChecker, worker.ErrorLogger)

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			<-ch

			signal.Stop(ch)
			s.GracefulStop()

			return errors.New("interrupt signal")
		})
	}
	{
		g.Go(func() error {
			// s.Serve() will create a service goroutine for each connection
			if err := s.Serve(lis); err != nil {
				return fmt.Errorf("failed to serve: %v", err)
			}
			return nil
		})
	}
	{
		g.Go(func() error {
			w.Run()
			return nil
		})
	}

	log.Println("Stopping Context-Box: ", g.Wait())
}
