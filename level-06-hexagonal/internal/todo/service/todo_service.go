// Package service is the application core: it implements the driving port
// (ports.TodoService) using only the driven ports.
//
// Domain errors are returned unwrapped so callers can match and show them;
// adapters already wrap infrastructure errors with context.
package service

import (
	"context"
	"time"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// TodoService implements ports.TodoService.
type TodoService struct {
	todos      ports.TodoRepository
	activities ports.ActivityRepository
	tx         ports.TxManager
}

var _ ports.TodoService = (*TodoService)(nil)

// NewTodoService returns the concrete type; Wire binds it to ports.TodoService.
func NewTodoService(todos ports.TodoRepository, activities ports.ActivityRepository, tx ports.TxManager) *TodoService {
	return &TodoService{todos: todos, activities: activities, tx: tx}
}

// Create stores a new todo and its "created" activity in one transaction.
func (s *TodoService) Create(ctx context.Context, title string) (*domain.Todo, error) {
	now := nowUTC()
	todo, err := domain.NewTodo(title, now)
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.activities.Create(ctx, domain.NewActivity(todo.ID, domain.ActionCreated, now))
	})
	if err != nil {
		return nil, err
	}
	return todo, nil
}

// Get returns one todo.
func (s *TodoService) Get(ctx context.Context, id string) (*domain.Todo, error) {
	return s.todos.Get(ctx, id)
}

// List returns a page of todos, newest first.
func (s *TodoService) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update renames and/or completes a todo, recording one activity per real change.
func (s *TodoService) Update(ctx context.Context, in ports.UpdateTodoInput) (*domain.Todo, error) {
	// Input is checked before the transaction, so a bad patch never touches the database.
	if in.Title == nil && in.Completed == nil {
		return nil, domain.ErrEmptyPatch
	}
	if in.Title != nil {
		if _, err := domain.ValidateTitle(*in.Title); err != nil {
			return nil, err
		}
	}

	now := nowUTC()
	var todo *domain.Todo
	// The read is inside the transaction and locks the row, because the changes are computed from it.
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = s.todos.GetForUpdate(ctx, in.ID)
		if err != nil {
			return err
		}

		actions, err := applyChanges(todo, in, now)
		if err != nil {
			return err
		}
		if len(actions) == 0 {
			return nil // nothing changed: no write, no activity
		}

		if err := s.todos.Update(ctx, todo); err != nil {
			return err
		}
		for _, action := range actions {
			if err := s.activities.Create(ctx, domain.NewActivity(todo.ID, action, now)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return todo, nil
}

// Delete hard-deletes a todo and records its "deleted" activity. The activities are kept.
func (s *TodoService) Delete(ctx context.Context, id string) error {
	now := nowUTC()
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Delete(ctx, id); err != nil {
			return err
		}
		return s.activities.Create(ctx, domain.NewActivity(id, domain.ActionDeleted, now))
	})
}

// ListActivities returns a todo's audit log, oldest first. It does not check that the todo exists.
func (s *TodoService) ListActivities(ctx context.Context, todoID string) ([]*domain.Activity, error) {
	return s.activities.ListByTodo(ctx, todoID)
}

// applyChanges applies the requested fields and returns one action per real change, in order.
func applyChanges(todo *domain.Todo, in ports.UpdateTodoInput, now time.Time) ([]domain.Action, error) {
	var actions []domain.Action
	if in.Title != nil {
		renamed, err := todo.Rename(*in.Title, now)
		if err != nil {
			return nil, err
		}
		if renamed {
			actions = append(actions, domain.ActionRenamed)
		}
	}
	if in.Completed != nil {
		if action, changed := todo.SetCompleted(*in.Completed, now); changed {
			actions = append(actions, action)
		}
	}
	return actions, nil
}

// nowUTC is the one timestamp of an operation, so a change and its activities share it.
// It is truncated to milliseconds because Mongo stores milliseconds; this way both
// adapters round-trip exactly the same value.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
