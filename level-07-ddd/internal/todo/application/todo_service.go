// Package application is the application service: it implements the driving port
// (ports.TodoService) by orchestrating the domain. Every command follows the same steps:
// parse value objects -> load the aggregate -> call its behavior -> repository.Save.
// It holds no business rules (the aggregate does) and no transaction (Save is one).
//
// Domain errors are returned unwrapped so callers can match and show them;
// adapters already wrap infrastructure errors with context.
package application

import (
	"context"
	"time"

	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
)

// TodoService implements ports.TodoService.
type TodoService struct {
	todos      domain.TodoRepository
	activities ports.ActivityLog
}

var _ ports.TodoService = (*TodoService)(nil)

// NewTodoService returns the concrete type; Wire binds it to ports.TodoService.
func NewTodoService(todos domain.TodoRepository, activities ports.ActivityLog) *TodoService {
	return &TodoService{todos: todos, activities: activities}
}

// Create starts a new todo. Save stores it together with its TodoCreated event.
func (s *TodoService) Create(ctx context.Context, rawTitle string) (*domain.Todo, error) {
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

// Get returns one todo.
func (s *TodoService) Get(ctx context.Context, rawID string) (*domain.Todo, error) {
	id, err := domain.ParseTodoID(rawID)
	if err != nil {
		return nil, err
	}
	return s.todos.Get(ctx, id)
}

// List returns a page of todos, newest first.
func (s *TodoService) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update renames, completes or reopens a todo. The aggregate records one event per real change.
func (s *TodoService) Update(ctx context.Context, in ports.UpdateTodoInput) (*domain.Todo, error) {
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

// Delete removes a todo. Its activities, including the new "deleted" one, are kept.
func (s *TodoService) Delete(ctx context.Context, rawID string) error {
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

// ListActivities returns a todo's audit log, oldest first. It does not check that the todo exists.
func (s *TodoService) ListActivities(ctx context.Context, rawTodoID string) ([]*domain.Activity, error) {
	todoID, err := domain.ParseTodoID(rawTodoID)
	if err != nil {
		return nil, err
	}
	return s.activities.ListByTodo(ctx, todoID)
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
