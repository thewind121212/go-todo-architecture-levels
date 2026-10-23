// Package ports declares the hexagon's boundaries as interfaces.
//
// Driving (primary) port: TodoService. Adapters on the left side, like HTTP,
// call it to drive the application.
//
// Driven (secondary) port: ActivityLog. The application calls it; adapters on
// the right side, like Postgres and Mongo, implement it. The other driven port,
// TodoRepository, is declared in domain: in DDD the repository is part of the model.
package ports

import (
	"context"

	"example.com/todo/level07/internal/todo/domain"
)

// TodoService is the driving port: everything the outside world may ask of the todo feature.
// Callers pass ids as they received them: the application parses them into a domain.TodoID.
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
