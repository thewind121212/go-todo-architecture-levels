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

// Service implements ports.TodoQueries. The compile-time check is in the module's
// newModule: ports imports this package for the view types, so this one cannot import ports.
type Service struct {
	todos TodoViews
}

func NewService(todos TodoViews) *Service {
	return &Service{todos: todos}
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
