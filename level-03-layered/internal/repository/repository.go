// Package repository defines the storage contracts the service depends on.
//
// In a classic layered architecture the repository layer owns these
// interfaces, and each database gets an adapter in a subpackage (postgres,
// mongo). Repositories are deliberately dumb: one collection each, plain CRUD,
// no business rules and no transactions of their own.
package repository

import (
	"context"

	"example.com/todo/level03/internal/model"
)

// TodoRepository stores todos. Get, Update and Delete return model.ErrNotFound
// for an unknown id.
type TodoRepository interface {
	Create(ctx context.Context, todo model.Todo) error
	Get(ctx context.Context, id string) (model.Todo, error)
	List(ctx context.Context, limit, offset int) ([]model.Todo, error) // newest first
	Update(ctx context.Context, todo model.Todo) error
	Delete(ctx context.Context, id string) error
}

// ActivityRepository stores the append-only activity log.
type ActivityRepository interface {
	Create(ctx context.Context, activity model.Activity) error
	ListByTodo(ctx context.Context, todoID string) ([]model.Activity, error) // oldest first
}

// TxManager runs fn inside one database transaction. fn must use the ctx it
// receives: that ctx carries the transaction, and the repositories pick it up
// from there. If fn returns an error, everything it wrote is rolled back.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
