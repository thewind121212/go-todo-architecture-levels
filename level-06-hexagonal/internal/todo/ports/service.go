// Package ports declares the hexagon's boundaries as interfaces.
//
// Driving (primary) port: TodoService. Adapters on the left side, like HTTP,
// call it to drive the application.
//
// Driven (secondary) ports: TodoRepository, ActivityRepository, TxManager.
// The application calls them; adapters on the right side, like Postgres and
// Mongo, implement them.
package ports

import (
	"context"

	"example.com/todo/level06/internal/todo/domain"
)

// TodoService is the driving port: everything the outside world may ask of the todo feature.
// Callers pass ids in canonical UUID form; the driving adapter is the one that parses them.
type TodoService interface {
	Create(ctx context.Context, title string) (*domain.Todo, error)
	Get(ctx context.Context, id string) (*domain.Todo, error)
	List(ctx context.Context, limit, offset int) ([]*domain.Todo, error)
	Update(ctx context.Context, in UpdateTodoInput) (*domain.Todo, error)
	Delete(ctx context.Context, id string) error
	ListActivities(ctx context.Context, todoID string) ([]*domain.Activity, error)
}

// UpdateTodoInput uses pointers because a nil field means "leave it unchanged".
type UpdateTodoInput struct {
	ID        string
	Title     *string
	Completed *bool
}
