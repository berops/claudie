package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/sanitise"
	"github.com/rs/zerolog/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	databaseName   = "claudie"
	collectionName = "inputManifests"
	pingTimeout    = 5 * time.Second
)

var _ Store = (*Mongo)(nil)

type Mongo struct {
	conn       *mongo.Client
	collection *mongo.Collection
}

func NewMongoClient(ctx context.Context, uri string) (*Mongo, error) {
	opts := options.Client().ApplyURI(uri)
	conn, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %q: %w", sanitise.URI(uri), err)
	}
	return &Mongo{conn: conn}, nil
}

func (m *Mongo) Close() error { return m.conn.Disconnect(context.Background()) }

func (m *Mongo) HealthCheck() error {
	ctx, done := context.WithTimeout(context.Background(), pingTimeout)
	defer done()
	return m.conn.Ping(ctx, readpref.Primary())
}

func (m *Mongo) Init() error {
	m.collection = m.conn.Database(databaseName).Collection(collectionName)
	index, err := m.collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create index: %q: %w", index, err)
	}
	return nil
}

func (m *Mongo) GetConfig(ctx context.Context, name string) (*Config, error) {
	result := m.collection.FindOne(ctx, bson.M{"name": name})

	if err := result.Err(); errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFoundOrDirty
	} else if err != nil {
		return nil, fmt.Errorf("failed to query document %q: %w", name, err)
	}

	var db Config
	if err := result.Decode(&db); err != nil {
		return nil, fmt.Errorf("failed to decode config %q: %w", name, err)
	}

	return &db, nil
}

func (m *Mongo) ListConfigs(ctx context.Context, filter *ListFilter) ([]*Config, error) {
	f := primitive.D{}

	if filter != nil && len(filter.ManifestState) > 0 {
		f = append(f, bson.E{
			Key:   "manifest.state",
			Value: bson.M{"$in": filter.ManifestState},
		})
	}

	cursor, err := m.collection.Find(ctx, f)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Err(err).Msgf("failed to close mongoDB cursor")
		}
	}()

	var out []*Config

	for {
		ok := cursor.Next(ctx)
		if ok {
			var db Config
			if err := cursor.Decode(&db); err != nil {
				return nil, fmt.Errorf("failed to decode db config: %w", err)
			}
			out = append(out, &db)
			continue
		}

		if err := cursor.Err(); err != nil {
			return nil, fmt.Errorf("failed to advance document cursor: %w", err)
		}

		if cursor.ID() == 0 {
			break
		}
	}

	return out, nil
}

func (m *Mongo) CreateConfig(ctx context.Context, config *Config) error {
	config.Version = 0                                // A new document always starts with 0 version.
	config.Manifest.State = manifest.Pending.String() // A new document always starts with the Pending state

	result, err := m.collection.InsertOne(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to insert config %q with version %v: %w", config.Name, config.Version, err)
	}

	log.Debug().Msgf("Inserted config %q with version %v under id %q", config.Name, config.Version, result.InsertedID)
	return nil
}

func (m *Mongo) UpdateConfig(ctx context.Context, config *Config) error {
	suppliedVersion := config.Version
	config.Version += 1

	result := m.collection.FindOneAndUpdate(
		ctx,
		bson.D{{Key: "name", Value: config.Name}, {Key: "version", Value: suppliedVersion}},
		bson.D{{Key: "$set", Value: config}},
	)

	if err := result.Err(); errors.Is(err, mongo.ErrNoDocuments) {
		config.Version = suppliedVersion
		return fmt.Errorf("failed to update config %q with version %v: %w", config.Name, suppliedVersion, ErrNotFoundOrDirty)
	} else if err != nil {
		config.Version = suppliedVersion
		return fmt.Errorf("failed to update config %q with version %v: %w", config.Name, suppliedVersion, err)
	}

	log.Debug().Msgf("Successfully Updated config %q with version %v, new version: %v", config.Name, suppliedVersion, config.Version)
	return nil
}

func (m *Mongo) DeleteConfig(ctx context.Context, name string, version uint64) error {
	result, err := m.collection.DeleteOne(ctx, bson.D{{Key: "name", Value: name}, {Key: "version", Value: version}})
	if err != nil {
		return fmt.Errorf("failed to delete config %q with version %v: %w", name, version, err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("failed to delete config %q under version %v: %w", name, version, ErrNotFoundOrDirty)
	}

	log.Info().Msgf("Document %q with version %v was deleted from database", name, version)
	return nil
}

func (m *Mongo) MarkForDeletion(ctx context.Context, name string, version uint64) error {
	result := m.collection.FindOneAndUpdate(
		ctx,
		bson.D{{Key: "name", Value: name}, {Key: "version", Value: version}},
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "manifest.raw", Value: ""},
				{Key: "manifest.checksum", Value: nil},
				{Key: "manifest.lastAppliedChecksum", Value: hash.Digest("delete")}}, // modify the last applied checksum so that repeated deletion will trigger the process again.
			},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
		},
	)

	if err := result.Err(); errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("failed to mark config %q with version %v for deletion: %w", name, version, ErrNotFoundOrDirty)
	} else if err != nil {
		return fmt.Errorf("failed to mark config %q with version %v for deletion: %w", name, version, err)
	}

	log.Debug().Msgf("Successfully marked config %q with version %v for deletion", name, version)

	return nil
}
