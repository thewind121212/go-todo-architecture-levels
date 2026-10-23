// Package mongo implements the repository interfaces with the MongoDB driver v2.
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

// Open connects to MongoDB. Transactions need a replica set, so uri must point at one.
func Open(uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	return client, nil
}

// Migrate creates the collections and the activities.todo_id index. Collections
// are created up front because creating one inside a multi-document
// transaction is not supported on every MongoDB version.
func Migrate(ctx context.Context, db *mongo.Database) error {
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

// TxManager starts MongoDB transactions for the service.
type TxManager struct {
	client *mongo.Client
}

func NewTxManager(client *mongo.Client) *TxManager {
	return &TxManager{client: client}
}

// WithinTx is where a transaction starts. Unlike the Postgres adapter, there
// is nothing to put into ctx by hand: driver v2 passes fn a ctx that already
// carries the session, and every collection call made with that ctx joins the
// transaction. So the repositories here just use the ctx they are given.
//
// WithTransaction commits when fn returns nil, aborts when it returns an
// error, and may re-run fn on transient errors, so fn must not have side
// effects outside the database.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}
