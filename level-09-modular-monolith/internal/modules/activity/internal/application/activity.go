// Package application is the whole inside of the activity module: its model, its ports and
// its service. The module is small, so it is one package: a module picks the architecture its
// own job needs, and nothing here has the rules that made the todo module DDD and CQRS.
package application

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is the activity module's "todo not found", for an id that is not a UUID.
// The module cannot use the todo module's errors: it may import only its contracts.
var ErrNotFound = errors.New("todo not found")

// Activity is one line of a todo's audit log, made from one todo event. Its id is the
// event's id, which is what makes recording an event twice harmless.
type Activity struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time // when the event occurred
}

// ActivityQueries is the module's driving port: what its HTTP adapter may ask.
type ActivityQueries interface {
	ListActivities(ctx context.Context, todoID string) ([]*Activity, error)
}

// ActivityLog stores the module's activities. The Postgres and Mongo adapters implement it.
type ActivityLog interface {
	ListByTodo(ctx context.Context, todoID string) ([]*Activity, error) // oldest first
	// Add inserts the activity unless one with its id exists: a repeat changes nothing.
	Add(ctx context.Context, activity *Activity) error
}

// TxManager runs fn in one transaction of the activity module's own storage and hands it
// to the adapters through ctx.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
