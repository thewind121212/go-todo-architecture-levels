package todo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level04/internal/platform/database"
)

const mongoCollection = "todos"

// MigrateMongo creates the todos collection. _id is the only index it needs.
func MigrateMongo(ctx context.Context, db *mongo.Database) error {
	return database.EnsureCollection(ctx, db, mongoCollection)
}

// MongoRepository needs no tx helper: collection calls made with the ctx from
// WithinTx join the session's transaction automatically.
type MongoRepository struct {
	coll *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{coll: db.Collection(mongoCollection)}
}

func (r *MongoRepository) Create(ctx context.Context, t Todo) error {
	if _, err := r.coll.InsertOne(ctx, t); err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *MongoRepository) Get(ctx context.Context, id string) (Todo, error) {
	var t Todo
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return t, nil
}

func (r *MongoRepository) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cur, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find todos: %w", err)
	}
	todos := []Todo{}
	if err := cur.All(ctx, &todos); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	return todos, nil
}

func (r *MongoRepository) Update(ctx context.Context, t Todo) error {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "title", Value: t.Title},
		{Key: "completed", Value: t.Completed},
		{Key: "updated_at", Value: t.UpdatedAt},
	}}}
	res, err := r.coll.UpdateByID(ctx, t.ID, update)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MongoRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}
