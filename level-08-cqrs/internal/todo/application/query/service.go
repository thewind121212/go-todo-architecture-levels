// Package query is the read side of CQRS. Its queries read only the read models (views),
// which the projector keeps up to date from the outbox. It never loads an aggregate and does
// not import the domain: the views are its own model, shaped for the API, not for the rules.
//
// Views lag the write side by up to one relay poll, so a todo just created may not be found
// for a moment. See the README on eventual consistency.
package query

import (
	"context"

	"github.com/google/uuid"
)

// Service implements ports.TodoQueries. The compile-time check is the wire.Bind in
// internal/wire: ports imports this package for the view types, so this one cannot import ports.
type Service struct {
	todos      TodoViews
	activities ActivityViews
}

func NewService(todos TodoViews, activities ActivityViews) *Service {
	return &Service{todos: todos, activities: activities}
}

// GetTodo returns one todo view.
func (s *Service) GetTodo(ctx context.Context, rawID string) (*TodoView, error) {
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	return s.todos.Get(ctx, id)
}

// ListTodos returns a page of todo views, newest first.
func (s *Service) ListTodos(ctx context.Context, limit, offset int) ([]*TodoView, error) {
	return s.todos.List(ctx, limit, offset)
}

// ListActivities returns a todo's audit log, oldest first. It does not check that the todo exists.
func (s *Service) ListActivities(ctx context.Context, rawTodoID string) ([]*ActivityView, error) {
	todoID, err := parseID(rawTodoID)
	if err != nil {
		return nil, err
	}
	return s.activities.ListByTodo(ctx, todoID)
}

// parseID returns the canonical form of a UUID. A malformed id names no todo, so it is
// ErrNotFound. This repeats domain.ParseTodoID's one rule on purpose: the read side must not
// depend on the write model, and the views are keyed by UUID too.
func parseID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", ErrNotFound
	}
	return id.String(), nil
}
