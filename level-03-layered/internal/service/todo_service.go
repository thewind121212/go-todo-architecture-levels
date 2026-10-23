// Package service holds the business flows. It is the only layer that knows
// which writes belong together, so it is the one that opens transactions.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"example.com/todo/level03/internal/model"
	"example.com/todo/level03/internal/repository"
)

type TodoService struct {
	tx         repository.TxManager
	todos      repository.TodoRepository
	activities repository.ActivityRepository
}

func NewTodoService(tx repository.TxManager, todos repository.TodoRepository, activities repository.ActivityRepository) *TodoService {
	return &TodoService{tx: tx, todos: todos, activities: activities}
}

func (s *TodoService) Create(ctx context.Context, title string) (model.Todo, error) {
	now := now()
	todo, err := model.NewTodo(title, now)
	if err != nil {
		return model.Todo{}, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, now, model.ActionCreated)
	})
	if err != nil {
		return model.Todo{}, err
	}
	return todo, nil
}

func (s *TodoService) Get(ctx context.Context, id string) (model.Todo, error) {
	if !validID(id) {
		return model.Todo{}, model.ErrNotFound
	}
	return s.todos.Get(ctx, id)
}

func (s *TodoService) List(ctx context.Context, limit, offset int) ([]model.Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update reads the todo, applies the patch, saves it and records one activity
// per change, all in one transaction: if any step fails, nothing is written.
func (s *TodoService) Update(ctx context.Context, id string, patch model.Patch) (model.Todo, error) {
	if !validID(id) {
		return model.Todo{}, model.ErrNotFound
	}
	var todo model.Todo
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = s.todos.Get(ctx, id)
		if err != nil {
			return err
		}
		now := now()
		actions, err := todo.Apply(patch, now)
		if err != nil {
			return err
		}
		if len(actions) == 0 {
			return nil // nothing changed, so nothing to write
		}
		if err := s.todos.Update(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, now, actions...)
	})
	if err != nil {
		return model.Todo{}, err
	}
	return todo, nil
}

func (s *TodoService) Delete(ctx context.Context, id string) error {
	if !validID(id) {
		return model.ErrNotFound
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, id, now(), model.ActionDeleted)
	})
}

// ListActivities does not check that the todo exists: activities outlive
// deleted todos.
func (s *TodoService) ListActivities(ctx context.Context, todoID string) ([]model.Activity, error) {
	if !validID(todoID) {
		return nil, nil
	}
	return s.activities.ListByTodo(ctx, todoID)
}

// record inserts one activity per action. It must be called inside WithinTx
// with the transaction's ctx.
func (s *TodoService) record(ctx context.Context, todoID string, now time.Time, actions ...model.Action) error {
	for _, action := range actions {
		activity, err := model.NewActivity(todoID, action, now)
		if err != nil {
			return err
		}
		if err := s.activities.Create(ctx, activity); err != nil {
			return err
		}
	}
	return nil
}

// now is truncated to milliseconds because that is all MongoDB stores; this
// way both adapters return exactly the time that was written.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// validID reports whether id is a UUID in the canonical lowercase form the app
// generates. Any other id cannot exist, so the service answers ErrNotFound
// without asking the database (Postgres would reject it as invalid uuid syntax).
func validID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == id
}
