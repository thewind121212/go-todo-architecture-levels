// Package usecase holds the application business rules: one interactor per use case. It depends only
// on entity. Everything it needs from the outside world is declared here as an interface (an output
// port), and the adapters implement it. That is the Dependency Rule: source code dependencies point
// inward, so the database depends on the use cases, never the other way round.
package usecase

import (
	"context"

	"example.com/todo/level05/internal/entity"
)

// TodoRepository stores todos. Get, GetForUpdate, Update and Delete return entity.ErrNotFound for an
// unknown id.
type TodoRepository interface {
	Create(ctx context.Context, todo entity.Todo) error
	Get(ctx context.Context, id string) (entity.Todo, error)
	// GetForUpdate reads a todo inside a transaction and keeps concurrent writers away from it
	// until the transaction ends.
	GetForUpdate(ctx context.Context, id string) (entity.Todo, error)
	List(ctx context.Context, limit, offset int) ([]entity.Todo, error) // newest first
	Update(ctx context.Context, todo entity.Todo) error
	Delete(ctx context.Context, id string) error
}

// ActivityRepository stores the append-only activity log.
type ActivityRepository interface {
	Create(ctx context.Context, activity entity.Activity) error
	ListByTodo(ctx context.Context, todoID string) ([]entity.Activity, error) // oldest first
}

// TxManager runs fn in one database transaction. The transaction travels inside the ctx given to fn,
// and the repositories pick it up from there. If fn returns an error, everything is rolled back.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
