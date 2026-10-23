// Package mongo implements the use case ports with the official MongoDB driver.
package mongo

import (
	"context"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	todosCollection      = "todos"
	activitiesCollection = "activities"
)

// Open connects to MongoDB and makes sure the collections and the index exist.
func Open(ctx context.Context, uri, dbName string) (*mongo.Database, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	db := client.Database(dbName)
	if err := createSchema(ctx, db); err != nil {
		return nil, err
	}
	return db, nil
}

// createSchema creates the collections up front, because not every MongoDB version can create a
// collection inside a multi-document transaction.
func createSchema(ctx context.Context, db *mongo.Database) error {
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range []string{todosCollection, activitiesCollection} {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection(activitiesCollection).Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}
