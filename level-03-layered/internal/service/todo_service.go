// Package service holds the business flows. It is the only layer that knows
// which writes belong together, so it is the one that opens transactions.
package service

import (
	"context"
	"time"

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
	todo, err := model.NewTodo(title, nowUTC())
	if err != nil {
		return model.Todo{}, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, todo.CreatedAt, model.ActionCreated)
	})
	if err != nil {
		return model.Todo{}, err
	}
	return todo, nil
}

func (s *TodoService) Get(ctx context.Context, id string) (model.Todo, error) {
	return s.todos.Get(ctx, id)
}

func (s *TodoService) List(ctx context.Context, limit, offset int) ([]model.Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update validates the patch first, so bad input never opens a transaction.
// Then one transaction locks and reads the todo, applies the patch, saves it
// and records one activity per change: if any step fails, nothing is written.
func (s *TodoService) Update(ctx context.Context, id string, p model.Patch) (model.Todo, error) {
	p, err := model.ValidatePatch(p)
	if err != nil {
		return model.Todo{}, err
	}
	var todo model.Todo
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = s.todos.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		now := nowUTC()
		actions := todo.Apply(p, now)
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
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, id, nowUTC(), model.ActionDeleted)
	})
}

// ListActivities does not check that the todo exists: activities outlive
// deleted todos.
func (s *TodoService) ListActivities(ctx context.Context, todoID string) ([]model.Activity, error) {
	return s.activities.ListByTodo(ctx, todoID)
}

// record inserts one activity per action. It must be called inside WithinTx
// with the transaction's ctx.
func (s *TodoService) record(ctx context.Context, todoID string, at time.Time, actions ...model.Action) error {
	for _, action := range actions {
		if err := s.activities.Create(ctx, model.NewActivity(todoID, action, at)); err != nil {
			return err
		}
	}
	return nil
}

// nowUTC is truncated to milliseconds because Mongo stores milliseconds;
// this way both adapters round-trip exactly the same value.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
