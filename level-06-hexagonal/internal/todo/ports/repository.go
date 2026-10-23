package ports

import (
	"context"

	"example.com/todo/level06/internal/todo/domain"
)

// TodoRepository is a driven port: the application stores todos through it.
// Get, GetForUpdate, Update and Delete return domain.ErrNotFound for an unknown id.
type TodoRepository interface {
	Create(ctx context.Context, todo *domain.Todo) error
	Get(ctx context.Context, id string) (*domain.Todo, error)
	// GetForUpdate reads a todo and locks it until the transaction in ctx ends.
	GetForUpdate(ctx context.Context, id string) (*domain.Todo, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) // newest first
	Update(ctx context.Context, todo *domain.Todo) error
	Delete(ctx context.Context, id string) error
}

// ActivityRepository is a driven port: the application appends to the audit log through it.
type ActivityRepository interface {
	Create(ctx context.Context, activity *domain.Activity) error
	ListByTodo(ctx context.Context, todoID string) ([]*domain.Activity, error) // oldest first
}

// TxManager is a driven port: it runs fn in one database transaction.
// The transaction travels inside the ctx given to fn; repositories pick it up
// from there, so the application never sees a *gorm.DB or a mongo session.
// If fn returns an error, everything is rolled back.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
