// Package command is the write side of CQRS: it implements the driving port
// ports.TodoCommands by orchestrating the domain. Every command follows the same steps:
// parse value objects -> load the aggregate -> call its behavior -> repository.Save.
// It holds no business rules (the aggregate does) and no transaction (Save is one).
// It never reads the views: the aggregate is the truth for changes.
//
// Domain errors are returned unwrapped so callers can match and show them;
// adapters already wrap infrastructure errors with context.
package command

import (
	"context"
	"time"

	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/ports"
)

// Service implements ports.TodoCommands.
type Service struct {
	todos domain.TodoRepository
}

var _ ports.TodoCommands = (*Service)(nil)

// NewService returns the concrete type; Wire binds it to ports.TodoCommands.
func NewService(todos domain.TodoRepository) *Service {
	return &Service{todos: todos}
}

// CreateTodo starts a new todo. Save stores it together with its TodoCreated event.
func (s *Service) CreateTodo(ctx context.Context, rawTitle string) (*domain.Todo, error) {
	title, err := domain.NewTitle(rawTitle)
	if err != nil {
		return nil, err
	}
	todo := domain.NewTodo(domain.NewTodoID(), title, nowUTC())
	if err := s.todos.Save(ctx, todo); err != nil {
		return nil, err
	}
	return todo, nil
}

// UpdateTodo renames, completes or reopens a todo. The aggregate records one event per real change.
func (s *Service) UpdateTodo(ctx context.Context, in ports.UpdateTodoInput) (*domain.Todo, error) {
	// All input becomes value objects before anything is loaded, so bad input never
	// touches the database.
	id, err := domain.ParseTodoID(in.ID)
	if err != nil {
		return nil, err
	}
	if in.Title == nil && in.Completed == nil {
		return nil, domain.ErrEmptyPatch
	}
	var title *domain.Title
	if in.Title != nil {
		t, err := domain.NewTitle(*in.Title)
		if err != nil {
			return nil, err
		}
		title = &t
	}

	todo, err := s.todos.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	applyChanges(todo, title, in.Completed, nowUTC())
	// Save fails with ErrConflict if another request saved this todo since Get.
	if err := s.todos.Save(ctx, todo); err != nil {
		return nil, err
	}
	return todo, nil
}

// DeleteTodo removes a todo. Its activities, including the new "deleted" one, are kept.
func (s *Service) DeleteTodo(ctx context.Context, rawID string) error {
	id, err := domain.ParseTodoID(rawID)
	if err != nil {
		return err
	}
	todo, err := s.todos.Get(ctx, id)
	if err != nil {
		return err
	}
	todo.Delete(nowUTC())
	return s.todos.Save(ctx, todo)
}

// applyChanges maps a PATCH onto the aggregate's behavior. Whether something really
// changed, and which events that records, is the aggregate's decision.
func applyChanges(todo *domain.Todo, title *domain.Title, completed *bool, now time.Time) {
	if title != nil {
		todo.Rename(*title, now)
	}
	if completed != nil {
		if *completed {
			todo.Complete(now)
		} else {
			todo.Reopen(now)
		}
	}
}

// nowUTC is the one timestamp of an operation, so a change and its events share it.
// It is truncated to milliseconds because Mongo stores milliseconds; this way both
// adapters round-trip exactly the same value.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
