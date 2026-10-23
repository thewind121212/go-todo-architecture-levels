package activity

import "context"

// Repository stores activities. Implementations join the transaction carried
// by ctx (see platform/tx).
type Repository interface {
	Create(ctx context.Context, a Activity) error
	ListByTodo(ctx context.Context, todoID string) ([]Activity, error)
}
