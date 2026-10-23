package database

import (
	"context"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Prepare creates one module's collections and indexes: Mongo's counterpart of that
// module's SQL migrations.
type Prepare func(ctx context.Context, db *mongo.Database) error

// OpenMongo connects, then lets each module prepare its own collections and indexes.
// The returned func disconnects the client.
func OpenMongo(ctx context.Context, uri, dbName string, modules ...Prepare) (*mongo.Database, func(), error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("connect mongo: %w", err)
	}
	disconnect := func() { _ = client.Disconnect(context.Background()) }
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		disconnect()
		return nil, nil, fmt.Errorf("ping mongo: %w", err)
	}
	db := client.Database(dbName)
	for _, prepare := range modules {
		if err := prepare(ctx, db); err != nil {
			disconnect()
			return nil, nil, err
		}
	}
	return db, disconnect, nil
}

// PingMongo checks that the database answers.
func PingMongo(ctx context.Context, db *mongo.Database) error {
	return db.Client().Ping(ctx, readpref.Primary())
}

// CreateCollections creates the named collections that do not exist yet. They are created
// up front because not every Mongo version can create them inside a multi-document transaction.
func CreateCollections(ctx context.Context, db *mongo.Database, names ...string) error {
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range names {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	return nil
}
