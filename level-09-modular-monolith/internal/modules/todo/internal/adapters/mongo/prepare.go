package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level09/internal/platform/database"
)

// Prepare creates the todo module's collections and indexes: Mongo's counterpart of the
// module's SQL migrations.
func Prepare(ctx context.Context, db *mongo.Database) error {
	if err := database.CreateCollections(ctx, db, todosCollection, outboxCollection, todoViewsCollection); err != nil {
		return err
	}
	// The relay's poll: unprocessed events in id order.
	unprocessed := mongo.IndexModel{Keys: bson.D{{Key: "processed_at", Value: 1}, {Key: "_id", Value: 1}}}
	if _, err := db.Collection(outboxCollection).Indexes().CreateOne(ctx, unprocessed); err != nil {
		return fmt.Errorf("create outbox index: %w", err)
	}
	return nil
}
