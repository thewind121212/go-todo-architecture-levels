package todo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level04/internal/platform/database"
)

const todosCollection = "todos"

type todoDoc struct {
	ID        string    `bson:"_id"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func toTodoDoc(t Todo) todoDoc {
	return todoDoc{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func (d todoDoc) toTodo() Todo {
	return Todo{ID: d.ID, Title: d.Title, Completed: d.Completed, CreatedAt: d.CreatedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC()}
}

// MigrateMongo creates this feature's collection. _id is the only index it needs.
func MigrateMongo(ctx context.Context, db *mongo.Database) error {
	return database.EnsureCollection(ctx, db, todosCollection)
}

// MongoRepository just uses the ctx it is given: inside WithinTx that ctx
// carries the session, so every call joins the transaction.
type MongoRepository struct {
	coll *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{coll: db.Collection(todosCollection)}
}

func (r *MongoRepository) Create(ctx context.Context, t Todo) error {
	if _, err := r.coll.InsertOne(ctx, toTodoDoc(t)); err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *MongoRepository) Get(ctx context.Context, id string) (Todo, error) {
	var doc todoDoc
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return doc.toTodo(), nil
}

// GetForUpdate is a plain find: a concurrent writer causes a write conflict,
// and WithTransaction retries the whole function.
func (r *MongoRepository) GetForUpdate(ctx context.Context, id string) (Todo, error) {
	return r.Get(ctx, id)
}

// List sorts by _id: UUIDv7 strings sort in creation order.
func (r *MongoRepository) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	var docs []todoDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	todos := make([]Todo, 0, len(docs))
	for _, doc := range docs {
		todos = append(todos, doc.toTodo())
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
