// Package service is the application core: it implements the driving port
// (ports.TodoService) using only the driven ports.
//
// Domain errors are returned unwrapped so callers can match and show them;
// adapters already wrap infrastructure errors with context.
package service

import (
	"context"

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
	todo, err := domain.NewTodo(title)
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, domain.ActionCreated)
	})
	if err != nil {
		return nil, err
	}
	return todo, nil
}

// Get returns one todo.
func (s *TodoService) Get(ctx context.Context, id string) (*domain.Todo, error) {
	if !domain.IsValidID(id) {
		return nil, domain.ErrNotFound
	}
	return s.todos.Get(ctx, id)
}

// List returns a page of todos, newest first.
func (s *TodoService) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update applies a partial update and records one activity per real change.
func (s *TodoService) Update(ctx context.Context, id string, in ports.UpdateTodoInput) (*domain.Todo, error) {
	if in.Title == nil && in.Completed == nil {
		return nil, domain.ErrEmptyUpdate
	}
	if !domain.IsValidID(id) {
		return nil, domain.ErrNotFound
	}
	var todo *domain.Todo
	// The read happens inside the transaction, so the change is based on what is committed.
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = s.todos.Get(ctx, id)
		if err != nil {
			return err
		}
		actions, err := applyUpdate(todo, in)
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
			if err := s.record(ctx, todo.ID, action); err != nil {
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

// Delete removes a todo and records a "deleted" activity in one transaction.
func (s *TodoService) Delete(ctx context.Context, id string) error {
	if !domain.IsValidID(id) {
		return domain.ErrNotFound
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, id, domain.ActionDeleted)
	})
}

// ListActivities returns a todo's audit log, oldest first. It does not check that the todo exists.
func (s *TodoService) ListActivities(ctx context.Context, todoID string) ([]*domain.Activity, error) {
	if !domain.IsValidID(todoID) {
		return []*domain.Activity{}, nil
	}
	return s.activities.ListByTodo(ctx, todoID)
}

// applyUpdate changes todo in memory and returns the actions to record, in order.
func applyUpdate(todo *domain.Todo, in ports.UpdateTodoInput) ([]domain.Action, error) {
	var actions []domain.Action
	if in.Title != nil {
		renamed, err := todo.Rename(*in.Title)
		if err != nil {
			return nil, err
		}
		if renamed {
			actions = append(actions, domain.ActionRenamed)
		}
	}
	if in.Completed != nil && todo.SetCompleted(*in.Completed) {
		actions = append(actions, domain.CompletionAction(todo.Completed))
	}
	return actions, nil
}

func (s *TodoService) record(ctx context.Context, todoID string, action domain.Action) error {
	activity, err := domain.NewActivity(todoID, action)
	if err != nil {
		return err
	}
	return s.activities.Append(ctx, activity)
}
