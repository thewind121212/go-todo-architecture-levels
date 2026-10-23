package query

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is the query side's own "todo not found": it cannot use domain.ErrNotFound,
// because the query side does not import the domain. The HTTP adapter maps both to 404.
var ErrNotFound = errors.New("todo not found")

// TodoView is a todo as the API shows it: one entry of the todo_views read model.
// It has no version and no behavior: it is data shaped for reading.
type TodoView struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TodoViews stores the todo_views read model. The query side owns it, so the interface
// lives here. Queries only read it; the projector is the only writer.
type TodoViews interface {
	// Get returns ErrNotFound for an unknown id.
	Get(ctx context.Context, id string) (*TodoView, error)
	// List is newest first.
	List(ctx context.Context, limit, offset int) ([]*TodoView, error)
	// Upsert inserts the view, or replaces the one with its id: doing it twice changes nothing.
	Upsert(ctx context.Context, view *TodoView) error
	// Delete removes the view. An unknown id is not an error, so a repeat changes nothing.
	Delete(ctx context.Context, id string) error
}
