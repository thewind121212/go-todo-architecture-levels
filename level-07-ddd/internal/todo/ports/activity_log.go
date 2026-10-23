package ports

import (
	"context"

	"example.com/todo/level07/internal/todo/domain"
)

// ActivityLog is a driven port for reading the audit log. It is read-only: activities
// are written only by domain.TodoRepository.Save, from the events the aggregate recorded.
type ActivityLog interface {
	ListByTodo(ctx context.Context, todoID domain.TodoID) ([]*domain.Activity, error) // oldest first
}
