package todo

import "context"

// Repository stores todos. Implementations join the transaction carried by
// ctx (see platform/tx).
type Repository interface {
	Create(ctx context.Context, t Todo) error
	Get(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, limit, offset int) ([]Todo, error)
	Update(ctx context.Context, t Todo) error
	Delete(ctx context.Context, id string) error
}
