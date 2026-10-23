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

// OpenMongo connects, then creates the collections and indexes if missing.
// The returned func disconnects the client.
func OpenMongo(ctx context.Context, uri, dbName string) (*mongo.Database, func(), error) {
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
	if err := prepareMongo(ctx, db); err != nil {
		disconnect()
		return nil, nil, err
	}
	return db, disconnect, nil
}

// PingMongo checks that the database answers.
func PingMongo(ctx context.Context, db *mongo.Database) error {
	return db.Client().Ping(ctx, readpref.Primary())
}

// prepareMongo is Mongo's counterpart of the SQL migrations. Collections are
// created up front because not every Mongo version can create them inside a
// multi-document transaction.
func prepareMongo(ctx context.Context, db *mongo.Database) error {
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range []string{"todos", "activities"} {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection("activities").Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}
