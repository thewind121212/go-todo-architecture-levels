package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// TodoRepository implements ports.TodoRepository.
// It uses ctx as given: inside WithinTx that ctx carries the session.
type TodoRepository struct {
	coll *mongo.Collection
}

var _ ports.TodoRepository = (*TodoRepository)(nil)

func NewTodoRepository(db *mongo.Database) *TodoRepository {
	return &TodoRepository{coll: db.Collection(todosCollection)}
}

func (r *TodoRepository) Create(ctx context.Context, todo *domain.Todo) error {
	if _, err := r.coll.InsertOne(ctx, toTodoDoc(todo)); err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) Get(ctx context.Context, id string) (*domain.Todo, error) {
	var d todoDoc
	err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find todo: %w", err)
	}
	return toDomainTodo(&d), nil
}

// GetForUpdate is a plain find: a concurrent writer causes a write conflict and WithTransaction retries.
func (r *TodoRepository) GetForUpdate(ctx context.Context, id string) (*domain.Todo, error) {
	return r.Get(ctx, id)
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	// UUIDv7 strings sort in creation order, so _id descending means newest first.
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cur, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find todos: %w", err)
	}
	var docs []todoDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	todos := make([]*domain.Todo, 0, len(docs))
	for i := range docs {
		todos = append(todos, toDomainTodo(&docs[i]))
	}
	return todos, nil
}

func (r *TodoRepository) Update(ctx context.Context, todo *domain.Todo) error {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "title", Value: todo.Title},
		{Key: "completed", Value: todo.Completed},
		{Key: "updated_at", Value: todo.UpdatedAt},
	}}}
	res, err := r.coll.UpdateByID(ctx, todo.ID, update)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
