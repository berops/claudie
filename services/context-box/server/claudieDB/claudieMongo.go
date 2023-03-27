package claudieDB

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"google.golang.org/protobuf/proto"
)

const (
	databaseName   = "claudie"
	collectionName = "inputManifests"
)

var (
	maxConnectionRetries = 20
	defaultPingTimeout   = 5 * time.Second
	defaultPingDelay     = 5 * time.Second
)

type ClaudieMongo struct {
	URL        string
	client     *mongo.Client
	collection *mongo.Collection
}

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
	ErrorMessage string             `bson:"errorMessage"`
}

// Connect tries to connect to the mongo DB until maxConnectionRetries reached
// if successful, returns mongo client, error otherwise
func (c *ClaudieMongo) Connect() error {
	// establish DB connection, this does not do any deployment checks/IO on the DB
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(c.URL))
	// safeURI represents the version of the connection string safe for
	// printing to console/logs.
	safeURI := utils.SanitiseURI(c.URL)

	if err != nil {
		return fmt.Errorf("failed to create a client at %s: %w", safeURI, err)
	} else {
		//if client creation successful, ping the DB to verify the connection
		for i := 0; i < maxConnectionRetries; i++ {
			log.Debug().Msgf("Trying to ping the DB at %s...", safeURI)
			err := pingTheDB(client)
			if err == nil {
				log.Debug().Msgf("The database at %s has been successfully pinged", safeURI)
				c.client = client
				return nil
			}
			// wait 5s for next retry
			time.Sleep(defaultPingDelay)
		}
		return fmt.Errorf("database connection at %s failed after %d attempts due to unsuccessful ping verification", safeURI, maxConnectionRetries)
	}
}

// Disconnect closes the connection to MongoDB
// returns error if closing was not successful
func (c *ClaudieMongo) Disconnect() error {
	return c.client.Disconnect(context.Background())
}

// Init will initialise database and collections
// returns error if initialisation failed, nil otherwise
func (c *ClaudieMongo) Init() error {
	c.collection = c.client.Database(databaseName).Collection(collectionName)
	// create index
	indexName, err := c.collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create index %s : %w", indexName, err)
	}
	return nil
}

// Delete config deletes a config from database permanently
// returns error if not successful, nil otherwise
func (c *ClaudieMongo) DeleteConfig(id string, idType pb.IdType) error {
	var filter primitive.M
	if idType == pb.IdType_HASH {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return fmt.Errorf("error while converting id %s to mongo primitive : %w", id, err)
		}
		filter = bson.M{"_id": oid} //create filter for searching in the database by hex id
	} else {
		filter = bson.M{"name": id} //create filter for searching in the database by name
	}

	res, err := c.collection.DeleteOne(context.Background(), filter) //delete object from the database
	if err != nil {
		return fmt.Errorf("error while attempting to delete config in MongoDB with ID %s : %w", id, err)
	}
	if res.DeletedCount == 0 { //check if the object was really deleted
		return fmt.Errorf("cannot find config with the specified ID %s : %w", id, err)
	}
	return nil
}

// GetConfig will get the config from the database, based on id and id type
// returns error if not successful, nil otherwise
func (c *ClaudieMongo) GetConfig(id string, idType pb.IdType) (*pb.Config, error) {
	var d configItem
	var err error
	if idType == pb.IdType_HASH {
		d, err = c.getByIDFromDB(id)
		if err != nil {
			return nil, err
		}
	} else {
		d, err = c.getByNameFromDB(id)
		if err != nil {
			return nil, err
		}
	}
	config, err := dataToConfigPb(&d)
	if err != nil {
		return nil, fmt.Errorf("error while converting config %s : %w", config.Name, err)
	}
	return config, nil
}

// GetAllConfig gets all configs from database
// returns slice of pb.Config if successful, error otherwise
func (c *ClaudieMongo) GetAllConfigs() ([]*pb.Config, error) {
	var res []*pb.Config             //slice of configs
	configs, err := c.getAllFromDB() //get all configs from database
	if err != nil {
		return nil, err
	}
	for _, config := range configs {
		//convert them to *pb.Config
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, fmt.Errorf("error while converting config %s : %w", config.Name, err)
		}
		res = append(res, c) // append them to result
	}
	return res, nil
}

// SaveConfig will save specified config in the database
// if config has been encountered before, based on id and name, it will update existing record
// return error if not successful, nil otherwise
func (c *ClaudieMongo) SaveConfig(config *pb.Config) error {
	// Convert desiredState and currentState to byte[] because we want to save them to the database
	var desiredStateByte, currentStateByte []byte
	var err error

	if desiredStateByte, err = proto.Marshal(config.DesiredState); err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}
	if currentStateByte, err = proto.Marshal(config.CurrentState); err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}

	// Parse data and map it to the configItem struct
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
	data.ErrorMessage = config.ErrorMessage

	// Check if ID exists
	// If config has already some ID:
	if config.GetId() != "" {
		//Get id from config as oid
		oid, err := primitive.ObjectIDFromHex(config.GetId())
		if err != nil {
			return fmt.Errorf("cannot parse ID %s : %w", config.Id, err)
		}
		filter := bson.M{"_id": oid}

		_, err = c.collection.ReplaceOne(context.Background(), filter, data)
		if err != nil {
			return fmt.Errorf("cannot update config with specified ID %s : %w", config.Id, err)
		}
	} else {
		// Add data to the collection if OID doesn't exist
		res, err := c.collection.InsertOne(context.Background(), data)
		if err != nil {
			// Return error in protobuf
			return fmt.Errorf("error while inserting config %s into DB: %w", config.Name, err)
		}

		oid, ok := res.InsertedID.(primitive.ObjectID)
		if !ok {
			return fmt.Errorf("error while getting oid for config %s : cannot convert to oid", config.Name)
		}
		data.ID = oid
		//set new id to config
		config.Id = oid.Hex()
	}
	return nil
}

// UpdateSchedulerTTL will update a schedulerTTL based on the name of the config
// returns error if not successful, nil otherwise
func (c *ClaudieMongo) UpdateSchedulerTTL(name string, newTTL int32) error {
	err := c.updateDocument(bson.M{"name": name}, bson.M{"$set": bson.M{"SchedulerTTL": newTTL}})
	if err != nil {
		return fmt.Errorf("failed to update Scheduler TTL for document %s : %w", name, err)
	}
	return nil
}

// UpdateBuilderTTL will update a builderTTL based on the name of the config
// returns error if not successful, nil otherwise
func (c *ClaudieMongo) UpdateBuilderTTL(name string, newTTL int32) error {
	err := c.updateDocument(bson.M{"name": name}, bson.M{"$set": bson.M{"BuilderTTL": newTTL}})
	if err != nil {
		return fmt.Errorf("failed to update Builder TTL for document %s : %w", name, err)
	}
	return nil
}

// UpdateMsToNull will update the msChecksum and manifest based on the id of the config
// returns error if not successful, nil otherwise
func (c *ClaudieMongo) UpdateMsToNull(hexId string) error {
	id, err := primitive.ObjectIDFromHex(hexId)
	if err != nil {
		return err
	}
	// update MsChecksum and manifest to null
	err = c.updateDocument(bson.M{"_id": id}, bson.M{"$set": bson.M{"manifest": nil, "msChecksum": nil, "errorMessage": nil}})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("document with id %s failed to update msChecksum : %w", id, err)
		}
		return err
	}
	return nil
}

// UpdateDs will update the desired state related field in DB
func (c *ClaudieMongo) UpdateDs(config *pb.Config) error {
	// convert DesiredState to []byte type
	desiredStateByte, err := proto.Marshal(config.DesiredState)
	if err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}
	// updation query
	err = c.updateDocument(bson.M{"name": config.Name}, bson.M{"$set": bson.M{
		"dsChecksum":   config.DsChecksum,
		"desiredState": desiredStateByte,
		"errorMessage": config.ErrorMessage,
	}})
	if err != nil {
		return fmt.Errorf("failed to update dsChecksum and desiredState for document %s : %w", config.Name, err)
	}
	return nil
}

// UpdateCs will update the current state related field in DB
func (c *ClaudieMongo) UpdateCs(config *pb.Config) error {
	// convert CurrentState to []byte type
	currentStateByte, err := proto.Marshal(config.CurrentState)
	if err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}
	err = c.updateDocument(bson.M{"name": config.Name}, bson.M{"$set": bson.M{
		"csChecksum":   config.CsChecksum,
		"currentState": currentStateByte,
		"errorMessage": config.ErrorMessage,
	}})
	if err != nil {
		return fmt.Errorf("failed to update csChecksum and currentState for document %s : %w", config.Name, err)
	}
	return nil
}

// getByNameFromDB will try to get a config from the database based on the name field
// returns config from database if successful, error otherwise
func (c *ClaudieMongo) getByNameFromDB(name string) (configItem, error) {
	var data configItem
	filter := bson.M{"name": name}
	if err := c.collection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		return data, fmt.Errorf("error while finding name %s in the DB: %w", name, err)
	}
	return data, nil
}

// getByIDFromDB will try to get a config from the database based on the id field
// returns config from database if successful, error otherwise
func (c *ClaudieMongo) getByIDFromDB(id string) (configItem, error) {
	var data configItem
	oid, err := primitive.ObjectIDFromHex(id) // convert id to mongo id type (oid)
	if err != nil {
		return data, fmt.Errorf("error while converting ID %s to oid : %w", id, err)
	}
	filter := bson.M{"_id": oid}
	if err := c.collection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		return data, fmt.Errorf("error while finding ID %s in the DB: %w", id, err)
	}
	return data, nil
}

// updateDocument will update at most one document from database based on the filter and operation
// returns error if not successful, nil otherwise
// return mongo.ErrNoDocuments if no document was found based on the filter
func (c *ClaudieMongo) updateDocument(filter, operation primitive.M) error {
	res := c.collection.FindOneAndUpdate(context.Background(), filter, operation)
	var r configItem
	err := res.Decode(&r)
	if err != nil {
		return err
	}
	return nil
}

// convert configItem struct to *pb.Config
// returns *pb.Config if successful, error otherwise
func dataToConfigPb(data *configItem) (*pb.Config, error) {
	var desiredState *pb.Project = new(pb.Project)
	err := proto.Unmarshal(data.DesiredState, desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling desiredState: %w", err)
	}

	var currentState *pb.Project = new(pb.Project)
	err = proto.Unmarshal(data.CurrentState, currentState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling currentState: %w", err)
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
		ErrorMessage: data.ErrorMessage,
	}, nil
}

// pingTheDB pings the mongo client connection
// returns nil if successful, error otherwise
func pingTheDB(client *mongo.Client) error {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), defaultPingTimeout)
	defer cancel()
	err := client.Ping(ctxWithTimeout, readpref.Primary())
	if err != nil {
		return fmt.Errorf("unable to ping the database: %w", err)
	}
	return nil
}

// getAllFromDB gets all configs from the database and returns slice of *configItem
func (c *ClaudieMongo) getAllFromDB() ([]*configItem, error) {
	var configs []*configItem
	cur, err := c.collection.Find(context.Background(), primitive.D{{}}) //primitive.D{{}} finds all records in the collection
	if err != nil {
		return nil, err
	}
	defer func() {
		err := cur.Close(context.Background())
		if err != nil {
			log.Error().Msgf("Failed to close MongoDB cursor: %v", err)
		}
	}()
	for cur.Next(context.Background()) { //Iterate through cur and extract all data
		data := &configItem{}
		err := cur.Decode(data) //Decode data from cursor to data
		if err != nil {
			return nil, fmt.Errorf("failed to decode data from cursor : %w", err)
		}
		configs = append(configs, data) //append decoded data (config) to res (response) slice
	}

	return configs, nil
}
