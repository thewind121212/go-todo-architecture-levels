package main

import "context"

// store is the only seam in this level: handler.go depends on it, and
// main.go picks the Postgres or Mongo implementation at startup.
// Each mutating method runs its own transaction.
type store interface {
	Create(ctx context.Context, title string) (Todo, error)
	List(ctx context.Context, limit, offset int) ([]Todo, error)
	Get(ctx context.Context, id string) (Todo, error)
	Update(ctx context.Context, id string, p todoPatch) (Todo, error)
	Delete(ctx context.Context, id string) error
	Activities(ctx context.Context, todoID string) ([]Activity, error)
	Ping(ctx context.Context) error
}
