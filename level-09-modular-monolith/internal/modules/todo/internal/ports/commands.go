// Package ports declares the driving ports: what the outside world, like the HTTP adapter,
// may ask of the todo feature. CQRS splits them in two: TodoCommands changes todos through
// the aggregate (write side), TodoQueries reads the views (read side).
//
// The driven ports live with the model they serve: domain.TodoRepository with the write
// model, query.TodoViews with the read model, projection.TxManager with the projector, and
// outbox.Store with the relay.
package ports

import (
	"context"

	"example.com/todo/level09/internal/modules/todo/internal/domain"
)

// TodoCommands is the write side's driving port. Callers pass ids as they received them:
// the command side parses them into a domain.TodoID.
//
// A command returns the aggregate it just saved, so POST and PATCH can answer with the new
// state at once instead of waiting for the read model to catch up.
type TodoCommands interface {
	CreateTodo(ctx context.Context, title string) (*domain.Todo, error)
	UpdateTodo(ctx context.Context, in UpdateTodoInput) (*domain.Todo, error)
	DeleteTodo(ctx context.Context, id string) error
}

// UpdateTodoInput uses pointers because a nil field means "leave it unchanged".
type UpdateTodoInput struct {
	ID        string
	Title     *string
	Completed *bool
}
