package outboundAdapters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"google.golang.org/protobuf/proto"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const (
	databaseName   = "claudie"
	collectionName = "inputManifests"

	maxConnectionRetriesCount = 20
	pingTimeout               = 5 * time.Second
	pingRetrialDelay          = 5 * time.Second
)

type MongoDBConnector struct {
	connectionUri    string
	client           *mongo.Client
	configCollection *mongo.Collection
}

type Workflow struct {
	Status      string `bson:"status"`
	Stage       string `bson:"stage"`
	Description string `bson:"description"`
}

type configItem struct {
	ID               primitive.ObjectID  `bson:"_id,omitempty"`
	Name             string              `bson:"name"`
	Manifest         string              `bson:"manifest"`
	DesiredState     []byte              `bson:"desiredState"`
	CurrentState     []byte              `bson:"currentState"`
	MsChecksum       []byte              `bson:"msChecksum"`
	DsChecksum       []byte              `bson:"dsChecksum"`
	CsChecksum       []byte              `bson:"csChecksum"`
	BuilderTTL       int                 `bson:"builderTTL"`
	SchedulerTTL     int                 `bson:"schedulerTTL"`
	State            map[string]Workflow `bson:"state"`
	ManifestFileName string              `bson:"manifestFileName"`
}

// NewMongoDBConnector creates a new instance of the MongoDBConnector struct
// retruns a pointer pointing to the new instance
func NewMongoDBConnector(connectionUri string) *MongoDBConnector {
	return &MongoDBConnector{
		connectionUri: connectionUri,
	}
}

// Connect tries to connect to MongoDB until maximum connection retries is reached
// If successful, returns mongo client, error otherwise
func (m *MongoDBConnector) Connect() error {
	// Establish DB connection, this does not do any deployment checks/IO on the DB
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(m.connectionUri))

	censoredUri := utils.SanitiseURI(m.connectionUri)

	if err != nil {
		return fmt.Errorf("Failed to create a mongoDB client with connection uri %s: %w", censoredUri, err)
	}

	for i := 0; i < maxConnectionRetriesCount; i++ {
		log.Debug().Msgf("Trying to ping mongoDB at %s", censoredUri)

		err := pingDB(client)
		if err == nil {
			log.Debug().Msgf("MongoDB at %s has been successfully pinged", censoredUri)

			m.client = client
			return nil
		}

		// wait for sometime before the next retry
		time.Sleep(pingRetrialDelay)
	}

	return fmt.Errorf("MongoDB connection at %s failed after %d unsuccessful ping attempts", censoredUri, maxConnectionRetriesCount)
}

// pingDB pings MongoDB and returns error (if any)
func pingDB(client *mongo.Client) error {
	contextWithTimeout, exitContext := context.WithTimeout(context.Background(), pingTimeout)
	defer exitContext()

	err := client.Ping(contextWithTimeout, readpref.Primary())
	if err != nil {
		return fmt.Errorf("Unable to ping mongoDB: %w", err)
	}

	return nil
}

// Init performs the initialization tasks after connection is established with MongoDB
func (m *MongoDBConnector) Init() error {
	m.configCollection = m.client.Database(databaseName).Collection(collectionName)

	indexName, err := m.configCollection.Indexes().CreateOne(context.Background(),
		mongo.IndexModel{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	)
	if err != nil {
		return fmt.Errorf("Failed to create index %s: %w", indexName, err)
	}

	return nil
}

// ConvertFromGRPCWorkflow converts the workflow state data from GRPC to the database representation.
func ConvertFromGRPCWorkflow(w map[string]*pb.Workflow) map[string]Workflow {
	state := make(map[string]Workflow, len(w))
	for key, val := range w {
		state[key] = Workflow{
			Status:      val.Status.String(),
			Stage:       val.Stage.String(),
			Description: val.Description,
		}
	}
	return state
}

// ConvertToGRPCWorkflow converts the database representation of the workflow state to GRPC.
func ConvertToGRPCWorkflow(w map[string]Workflow) map[string]*pb.Workflow {
	state := make(map[string]*pb.Workflow, len(w))
	for key, val := range w {
		state[key] = &pb.Workflow{
			Stage:       pb.Workflow_Stage(pb.Workflow_Stage_value[val.Stage]),
			Status:      pb.Workflow_Status(pb.Workflow_Status_value[val.Status]),
			Description: val.Description,
		}
	}
	return state
}

// Delete config deletes a config from database permanently
// returns error if not successful, nil otherwise
func (m *MongoDBConnector) DeleteConfig(id string, idType pb.IdType) error {
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

	res, err := m.configCollection.DeleteOne(context.Background(), filter) //delete object from the database
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
func (m *MongoDBConnector) GetConfig(id string, idType pb.IdType) (*pb.Config, error) {
	var d configItem
	var err error
	if idType == pb.IdType_HASH {
		d, err = m.getByIDFromDB(id)
		if err != nil {
			return nil, err
		}
	} else {
		d, err = m.getByNameFromDB(id)
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
func (m *MongoDBConnector) GetAllConfigs() ([]*pb.Config, error) {
	var res []*pb.Config             //slice of configs
	configs, err := m.getAllFromDB() //get all configs from database
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
func (m *MongoDBConnector) SaveConfig(config *pb.Config) error {
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
	data.State = ConvertFromGRPCWorkflow(config.State)
	data.ManifestFileName = config.GetManifestFileName()

	// Check if ID exists
	// If config has already some ID:
	if config.GetId() != "" {
		//Get id from config as oid
		oid, err := primitive.ObjectIDFromHex(config.GetId())
		if err != nil {
			return fmt.Errorf("cannot parse ID %s : %w", config.Id, err)
		}
		filter := bson.M{"_id": oid}

		_, err = m.configCollection.ReplaceOne(context.Background(), filter, data)
		if err != nil {
			return fmt.Errorf("cannot update config with specified ID %s : %w", config.Id, err)
		}
	} else {
		// Add data to the collection if OID doesn't exist
		res, err := m.configCollection.InsertOne(context.Background(), data)
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
func (m *MongoDBConnector) UpdateSchedulerTTL(name string, newTTL int32) error {
	err := m.updateDocument(bson.M{"name": name}, bson.M{"$set": bson.M{"schedulerTTL": newTTL}})
	if err != nil {
		return fmt.Errorf("failed to update Scheduler TTL for document %s : %w", name, err)
	}
	return nil
}

// UpdateBuilderTTL will update a builderTTL based on the name of the config
// returns error if not successful, nil otherwise
func (m *MongoDBConnector) UpdateBuilderTTL(name string, newTTL int32) error {
	err := m.updateDocument(bson.M{"name": name}, bson.M{"$set": bson.M{"builderTTL": newTTL}})
	if err != nil {
		return fmt.Errorf("failed to update Builder TTL for document %s : %w", name, err)
	}
	return nil
}

// UpdateMsToNull will update the msChecksum and manifest based on the id of the config
// returns error if not successful, nil otherwise
func (c *MongoDBConnector) UpdateMsToNull(id string, idType pb.IdType) error {
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
	// update MsChecksum and manifest to null
	err := c.updateDocument(filter, bson.M{"$set": bson.M{"manifest": nil, "msChecksum": nil, "state": map[string]Workflow{}}})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("document with id %s failed to update msChecksum : %w", id, err)
		}
		return err
	}
	return nil
}

// UpdateDs will update the desired state related field in DB
func (m *MongoDBConnector) UpdateDs(config *pb.Config) error {
	// convert DesiredState to []byte type
	desiredStateByte, err := proto.Marshal(config.DesiredState)
	if err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}
	// updation query
	err = m.updateDocument(bson.M{"name": config.Name}, bson.M{"$set": bson.M{
		"dsChecksum":   config.DsChecksum,
		"desiredState": desiredStateByte,
	}})
	if err != nil {
		return fmt.Errorf("failed to update dsChecksum and desiredState for document %s : %w", config.Name, err)
	}
	return nil
}

// UpdateWorkflowState updates the state of the config with the given workflow
func (m *MongoDBConnector) UpdateWorkflowState(configName, clusterName string, workflow *pb.Workflow) error {
	if workflow == nil {
		return nil
	}
	return m.updateDocument(bson.M{"name": configName}, bson.M{"$set": bson.M{
		fmt.Sprintf("state.%s", clusterName): Workflow{
			Status:      workflow.Status.String(),
			Stage:       workflow.Stage.String(),
			Description: workflow.Description,
		},
	}})
}

// UpdateAllStates updates all states of the config specified.
func (c *MongoDBConnector) UpdateAllStates(configName string, states map[string]*pb.Workflow) error {
	if states == nil {
		return nil
	}
	return c.updateDocument(bson.M{"name": configName}, bson.M{"$set": bson.M{"state": ConvertFromGRPCWorkflow(states)}})
}

// UpdateCs will update the current state related field in DB
func (m *MongoDBConnector) UpdateCs(config *pb.Config) error {
	// convert CurrentState to []byte type
	currentStateByte, err := proto.Marshal(config.CurrentState)
	if err != nil {
		return fmt.Errorf("error while converting config %s from protobuf to byte: %w", config.Name, err)
	}
	err = m.updateDocument(bson.M{"name": config.Name}, bson.M{"$set": bson.M{
		"csChecksum":   config.CsChecksum,
		"currentState": currentStateByte,
	}})
	if err != nil {
		return fmt.Errorf("failed to update csChecksum and currentState for document %s : %w", config.Name, err)
	}
	return nil
}

// getByNameFromDB will try to get a config from the database based on the name field
// returns config from database if successful, error otherwise
func (m *MongoDBConnector) getByNameFromDB(name string) (configItem, error) {
	var data configItem
	filter := bson.M{"name": name}
	if err := m.configCollection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		return data, fmt.Errorf("error while finding name %s in the DB: %w", name, err)
	}
	return data, nil
}

// getByIDFromDB will try to get a config from the database based on the id field
// returns config from database if successful, error otherwise
func (m *MongoDBConnector) getByIDFromDB(id string) (configItem, error) {
	var data configItem
	oid, err := primitive.ObjectIDFromHex(id) // convert id to mongo id type (oid)
	if err != nil {
		return data, fmt.Errorf("error while converting ID %s to oid : %w", id, err)
	}
	filter := bson.M{"_id": oid}
	if err := m.configCollection.FindOne(context.Background(), filter).Decode(&data); err != nil {
		return data, fmt.Errorf("error while finding ID %s in the DB: %w", id, err)
	}
	return data, nil
}

// updateDocument will update at most one document from database based on the filter and operation
// returns error if not successful, nil otherwise
// return mongo.ErrNoDocuments if no document was found based on the filter
func (m *MongoDBConnector) updateDocument(filter, operation primitive.M) error {
	res := m.configCollection.FindOneAndUpdate(context.Background(), filter, operation)
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
		Id:               data.ID.Hex(),
		Name:             data.Name,
		Manifest:         data.Manifest,
		DesiredState:     desiredState,
		CurrentState:     currentState,
		MsChecksum:       data.MsChecksum,
		DsChecksum:       data.DsChecksum,
		CsChecksum:       data.CsChecksum,
		BuilderTTL:       int32(data.BuilderTTL),
		SchedulerTTL:     int32(data.SchedulerTTL),
		State:            ConvertToGRPCWorkflow(data.State),
		ManifestFileName: data.ManifestFileName,
	}, nil
}

// getAllFromDB gets all configs from the database and returns slice of *configItem
func (m *MongoDBConnector) getAllFromDB() ([]*configItem, error) {
	var configs []*configItem
	cur, err := m.configCollection.Find(context.Background(), primitive.D{{}}) //primitive.D{{}} finds all records in the collection
	if err != nil {
		return nil, err
	}
	defer func() {
		err := cur.Close(context.Background())
		if err != nil {
			log.Err(err).Msgf("Failed to close MongoDB cursor")
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

// Disconnect closes the connection to MongoDB
// returns error if closing was not successful
func (m *MongoDBConnector) Disconnect() {
	err := m.client.Disconnect(context.Background())
	if err != nil {
		log.Error().Msgf("Error while closing the connection to MongoDB : %v", err)
	}
}
