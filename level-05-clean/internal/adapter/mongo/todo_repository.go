package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level05/internal/entity"
)

// TodoRepository implements usecase.TodoRepository.
type TodoRepository struct {
	todos *mongo.Collection
}

func NewTodoRepository(db *mongo.Database) *TodoRepository {
	return &TodoRepository{todos: db.Collection(todosCollection)}
}

func (r *TodoRepository) Create(ctx context.Context, todo entity.Todo) error {
	if _, err := r.todos.InsertOne(ctx, toTodoDocument(todo)); err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) FindByID(ctx context.Context, id string) (entity.Todo, error) {
	var doc todoDocument
	err := r.todos.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return entity.Todo{}, entity.ErrNotFound
	}
	if err != nil {
		return entity.Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return toTodoEntity(doc), nil
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]entity.Todo, error) {
	// UUIDv7 strings sort by time, so _id descending is newest first.
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.todos.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	var docs []todoDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	todos := make([]entity.Todo, 0, len(docs))
	for _, d := range docs {
		todos = append(todos, toTodoEntity(d))
	}
	return todos, nil
}

func (r *TodoRepository) Update(ctx context.Context, todo entity.Todo) error {
	res, err := r.todos.ReplaceOne(ctx, bson.D{{Key: "_id", Value: todo.ID}}, toTodoDocument(todo))
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if res.MatchedCount == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res, err := r.todos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if res.DeletedCount == 0 {
		return entity.ErrNotFound
	}
	return nil
}
