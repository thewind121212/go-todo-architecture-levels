package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// OpenMongo connects and pings MongoDB.
func OpenMongo(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongo: %w", err)
	}
	return client, nil
}

// EnsureCollection creates a collection if it does not exist yet. Features
// call it at startup because creating a collection inside a multi-document
// transaction is not supported on every MongoDB version.
func EnsureCollection(ctx context.Context, db *mongo.Database, name string) error {
	names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	if len(names) > 0 {
		return nil
	}
	if err := db.CreateCollection(ctx, name); err != nil {
		return fmt.Errorf("create collection %s: %w", name, err)
	}
	return nil
}
