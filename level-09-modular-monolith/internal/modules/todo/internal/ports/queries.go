package ports

import (
	"context"

	"example.com/todo/level09/internal/modules/todo/internal/application/query"
)

// TodoQueries is the read side's driving port. It returns views, not aggregates, and may
// lag the latest command by one relay poll.
type TodoQueries interface {
	GetTodo(ctx context.Context, id string) (*query.TodoView, error)
	ListTodos(ctx context.Context, limit, offset int) ([]*query.TodoView, error)
}
