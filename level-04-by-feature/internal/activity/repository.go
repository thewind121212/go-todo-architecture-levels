package activity

import "context"

// Repository stores the append-only activity log. Implementations join the
// transaction carried by ctx (see platform/tx).
type Repository interface {
	Create(ctx context.Context, a Activity) error
	ListByTodo(ctx context.Context, todoID string) ([]Activity, error) // oldest first
}
