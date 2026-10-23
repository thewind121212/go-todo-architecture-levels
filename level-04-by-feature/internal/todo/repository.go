package todo

import "context"

// Repository stores todos. Get, GetForUpdate, Update and Delete return
// ErrNotFound for an unknown id. Implementations join the transaction carried
// by ctx (see platform/tx).
type Repository interface {
	Create(ctx context.Context, t Todo) error
	Get(ctx context.Context, id string) (Todo, error)
	// GetForUpdate is Get for a read that a write depends on; call it inside WithinTx.
	GetForUpdate(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, limit, offset int) ([]Todo, error) // newest first
	Update(ctx context.Context, t Todo) error
	Delete(ctx context.Context, id string) error
}
