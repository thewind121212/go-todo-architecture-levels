package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level09/internal/platform/database"
)

// Prepare creates the activity module's collection and index: Mongo's counterpart of the
// module's SQL migrations.
func Prepare(ctx context.Context, db *mongo.Database) error {
	if err := database.CreateCollections(ctx, db, activitiesCollection); err != nil {
		return err
	}
	byTodo := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection(activitiesCollection).Indexes().CreateOne(ctx, byTodo); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}
