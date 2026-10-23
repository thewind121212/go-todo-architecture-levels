package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level07/internal/todo/domain"
)

// TodoRepository implements domain.TodoRepository.
type TodoRepository struct {
	client     *mongo.Client
	todos      *mongo.Collection
	activities *mongo.Collection
}

var _ domain.TodoRepository = (*TodoRepository)(nil)

func NewTodoRepository(db *mongo.Database) *TodoRepository {
	return &TodoRepository{
		client:     db.Client(),
		todos:      db.Collection(todosCollection),
		activities: db.Collection(activitiesCollection),
	}
}

func (r *TodoRepository) Get(ctx context.Context, id domain.TodoID) (*domain.Todo, error) {
	var d todoDoc
	err := r.todos.FindOne(ctx, bson.D{{Key: "_id", Value: id.String()}}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find todo: %w", err)
	}
	return toDomainTodo(&d), nil
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	// UUIDv7 strings sort in creation order, so _id descending means newest first.
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cur, err := r.todos.Find(ctx, bson.D{}, opts)
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

// Save is where the transaction lives. The aggregate is the consistency boundary, so it is
// also the transaction boundary: its new state and its events are committed together or
// not at all. The ctx that WithTransaction passes to its callback carries the session, and
// every collection call uses it, so all of them join the transaction. An error aborts it.
func (r *TodoRepository) Save(ctx context.Context, todo *domain.Todo) error {
	events := todo.PullEvents()
	if len(events) == 0 {
		return nil // nothing changed: no write, no activity
	}
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		if err := r.saveState(ctx, todo); err != nil {
			return nil, err
		}
		if _, err := r.activities.InsertMany(ctx, toActivityDocs(events)); err != nil {
			return nil, fmt.Errorf("insert activities: %w", err)
		}
		return nil, nil
	})
	return err
}

// saveState inserts a never-saved todo (version 0) as version 1, or replaces or deletes
// the stored document, writing version Version()+1.
func (r *TodoRepository) saveState(ctx context.Context, todo *domain.Todo) error {
	d := toTodoDoc(todo)
	if todo.Version() == 0 {
		if _, err := r.todos.InsertOne(ctx, d); err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		return nil
	}

	// Optimistic lock: only overwrite the document we loaded; if someone saved since, nothing matches.
	loaded := bson.D{{Key: "_id", Value: d.ID}, {Key: "version", Value: todo.Version()}}
	var matched int64
	if todo.Deleted() {
		res, err := r.todos.DeleteOne(ctx, loaded)
		if err != nil {
			return fmt.Errorf("delete todo: %w", err)
		}
		matched = res.DeletedCount
	} else {
		res, err := r.todos.ReplaceOne(ctx, loaded, d)
		if err != nil {
			return fmt.Errorf("replace todo: %w", err)
		}
		matched = res.MatchedCount
	}
	if matched == 0 {
		return domain.ErrConflict
	}
	return nil
}
