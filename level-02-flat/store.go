// store.go: the interface between the HTTP handlers and the databases.

package main

import "context"

// store is everything the HTTP handlers need from a database. Postgres and
// Mongo both implement it, and main picks one at startup.
type store interface {
	Create(ctx context.Context, t Todo) error
	Get(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, limit, offset int) ([]Todo, error)
	Update(ctx context.Context, id string, p Patch) (Todo, error)
	Delete(ctx context.Context, id string) error
	ListActivities(ctx context.Context, todoID string) ([]Activity, error)
	Ping(ctx context.Context) error
}
