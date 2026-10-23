package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level03/internal/model"
)

type todoDoc struct {
	ID        string    `bson:"_id"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (d todoDoc) toModel() model.Todo {
	return model.Todo{ID: d.ID, Title: d.Title, Completed: d.Completed, CreatedAt: d.CreatedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC()}
}

type TodoRepository struct {
	coll *mongo.Collection
}

func NewTodoRepository(db *mongo.Database) *TodoRepository {
	return &TodoRepository{coll: db.Collection(todosCollection)}
}

func (r *TodoRepository) Create(ctx context.Context, t model.Todo) error {
	doc := todoDoc{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) Get(ctx context.Context, id string) (model.Todo, error) {
	var doc todoDoc
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.Todo{}, model.ErrNotFound
	}
	if err != nil {
		return model.Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return doc.toModel(), nil
}

// List sorts by _id: UUIDv7 strings sort in creation order.
func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]model.Todo, error) {
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
	todos := make([]model.Todo, 0, len(docs))
	for _, doc := range docs {
		todos = append(todos, doc.toModel())
	}
	return todos, nil
}

func (r *TodoRepository) Update(ctx context.Context, t model.Todo) error {
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
		return model.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if res.DeletedCount == 0 {
		return model.ErrNotFound
	}
	return nil
}
