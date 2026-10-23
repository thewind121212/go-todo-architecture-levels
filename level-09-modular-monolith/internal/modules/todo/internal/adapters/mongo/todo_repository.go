package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level09/internal/modules/todo/internal/domain"
)

// TodoRepository implements domain.TodoRepository.
type TodoRepository struct {
	client *mongo.Client
	todos  *mongo.Collection
	outbox *mongo.Collection
}

var _ domain.TodoRepository = (*TodoRepository)(nil)

func NewTodoRepository(db *mongo.Database) *TodoRepository {
	return &TodoRepository{
		client: db.Client(),
		todos:  db.Collection(todosCollection),
		outbox: db.Collection(outboxCollection),
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

// Save is the write side's transaction. The aggregate is the consistency boundary, so it is
// also the transaction boundary: its new state and its events are committed together or
// not at all. The events go to the outbox, a collection in this same database, so one
// transaction covers it. The ctx that WithTransaction passes to its callback carries the
// session, and every collection call uses it, so all of them join the transaction.
// An error aborts it.
func (r *TodoRepository) Save(ctx context.Context, todo *domain.Todo) error {
	events := todo.PullEvents()
	if len(events) == 0 {
		return nil // nothing changed: no write, no event
	}
	messages, err := toOutboxDocs(events)
	if err != nil {
		return err
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
		if _, err := r.outbox.InsertMany(ctx, messages); err != nil {
			return nil, fmt.Errorf("insert outbox: %w", err)
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
