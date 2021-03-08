package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var collection *mongo.Collection

type server struct{}

type configItem struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"`
	Name    string             `bson:"name"`
	Content string             `bson:"content"`
}

func dataToConfigPb(data *configItem) *pb.Config {
	return &pb.Config{
		Id:      data.ID.Hex(),
		Name:    data.Name,
		Content: data.Content,
	}
}

func (*server) SaveConfig(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Println("Save config request")
	config := req.GetConfig()

	//Parse data and map it to configItem struct
	data := &configItem{}
	data.Name = config.GetName()
	data.Content = config.GetContent()

	//Check if ID exists
	if config.GetId() != "" {
		//Get id from config
		oid, err := primitive.ObjectIDFromHex(config.GetId())
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument,
				fmt.Sprintf("Cannot parse ID"),
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

		return &pb.SaveConfigResponse{Config: dataToConfigPb(data)}, nil
	}
	//Add data to the collection
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
			fmt.Sprintf("Cannot convert to OID"),
		)
	}
	data.ID = oid
	//Return config with ID
	return &pb.SaveConfigResponse{Config: dataToConfigPb(data)}, nil
}

func main() {
	// If we crath the go gode, we get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Connect to MongoDB
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017")) //client represents conection object do db
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB")
	collection = client.Database("platform").Collection("config")

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
		fmt.Println("Server started")
	}()

	// Wait for Control C to exit
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Block until a signal is received
	<-ch
	fmt.Println("Stopping the server")
	s.Stop()
	fmt.Println("Closing the listener")
	lis.Close()
	fmt.Println("Closing MongDB Connection")
	client.Disconnect(context.TODO())
	fmt.Println("End of Program")
}
