package ports

import (
	"context"

	"example.com/todo/level06/internal/todo/domain"
)

// TodoRepository is a driven port: the application stores todos through it.
// Get, Update and Delete return domain.ErrNotFound when no todo has the id.
type TodoRepository interface {
	Create(ctx context.Context, todo *domain.Todo) error
	Get(ctx context.Context, id string) (*domain.Todo, error)
	// List returns todos newest first.
	List(ctx context.Context, limit, offset int) ([]*domain.Todo, error)
	Update(ctx context.Context, todo *domain.Todo) error
	Delete(ctx context.Context, id string) error
}

// ActivityRepository is a driven port: the application appends to the audit log through it.
type ActivityRepository interface {
	Append(ctx context.Context, activity *domain.Activity) error
	// ListByTodo returns a todo's activities oldest first.
	ListByTodo(ctx context.Context, todoID string) ([]*domain.Activity, error)
}

// TxManager is a driven port: it runs fn in one database transaction.
// The transaction travels inside the ctx given to fn; repositories pick it up
// from there, so the application never sees a *gorm.DB or a mongo session.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
