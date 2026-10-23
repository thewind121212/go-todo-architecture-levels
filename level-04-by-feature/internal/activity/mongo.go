package activity

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level04/internal/platform/database"
)

const mongoCollection = "activities"

// MigrateMongo creates the activities collection and its todo_id index.
func MigrateMongo(ctx context.Context, db *mongo.Database) error {
	if err := database.EnsureCollection(ctx, db, mongoCollection); err != nil {
		return err
	}
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection(mongoCollection).Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}

// MongoRepository needs no tx helper: collection calls made with the ctx from
// WithinTx join the session's transaction automatically.
type MongoRepository struct {
	coll *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{coll: db.Collection(mongoCollection)}
}

func (r *MongoRepository) Create(ctx context.Context, a Activity) error {
	if _, err := r.coll.InsertOne(ctx, a); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *MongoRepository) ListByTodo(ctx context.Context, todoID string) ([]Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := r.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("find activities: %w", err)
	}
	activities := []Activity{}
	if err := cur.All(ctx, &activities); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	return activities, nil
}
