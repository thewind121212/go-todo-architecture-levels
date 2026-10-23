// Package projection keeps the todo module's read model up to date. It is where the two
// sides of CQRS meet: it reads the module's published events from the bus and writes the
// query side's views.
package projection

import (
	"context"
	"fmt"

	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/platform/eventbus"
)

// TxManager runs fn in one transaction of the todo module's database and hands it to the
// adapters through ctx. The projector owns its transaction: the relay that delivered the
// event has none to offer, and no other module's transaction may be joined.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Projector is a subscriber on the event bus, like any other module's. It listens to the
// todo module's own contract events, not to its domain events: one way out of the module
// for everyone, and proof that the contract carries what a read model needs.
//
// Every write it makes is idempotent (upsert by id, delete), so applying an event twice
// leaves the view as applying it once. That is what makes at-least-once delivery safe.
type Projector struct {
	todos query.TodoViews
	tx    TxManager
}

func NewProjector(todos query.TodoViews, tx TxManager) *Projector {
	return &Projector{todos: todos, tx: tx}
}

// Subscribe registers Handle for every todo event.
func (p *Projector) Subscribe(bus *eventbus.Bus) {
	for _, name := range []string{
		contracts.TodoCreatedName, contracts.TodoRenamedName, contracts.TodoCompletedName,
		contracts.TodoReopenedName, contracts.TodoDeletedName,
	} {
		bus.Subscribe(name, p.Handle)
	}
}

// Handle applies one event to the todo view, in the todo module's own transaction: the
// load and the upsert of an update commit together.
func (p *Projector) Handle(ctx context.Context, event eventbus.Event) error {
	return p.tx.WithinTx(ctx, func(ctx context.Context) error {
		return p.projectTodo(ctx, event)
	})
}

// projectTodo turns the event into the todo view's new state.
func (p *Projector) projectTodo(ctx context.Context, event eventbus.Event) error {
	switch e := event.(type) {
	case contracts.TodoCreated:
		return p.todos.Upsert(ctx, &query.TodoView{
			ID:        e.TodoID,
			Title:     e.Title,
			CreatedAt: e.OccurredAt,
			UpdatedAt: e.OccurredAt,
		})
	case contracts.TodoRenamed:
		return p.update(ctx, e.EventMeta, func(v *query.TodoView) { v.Title = e.Title })
	case contracts.TodoCompleted:
		return p.update(ctx, e.EventMeta, func(v *query.TodoView) { v.Completed = true })
	case contracts.TodoReopened:
		return p.update(ctx, e.EventMeta, func(v *query.TodoView) { v.Completed = false })
	case contracts.TodoDeleted:
		return p.todos.Delete(ctx, e.TodoID)
	default:
		return fmt.Errorf("project todo: unknown event %T", event)
	}
}

// update loads the view, changes it, and upserts it with the event's time as updated_at.
// Events arrive in order, so the view it loads already has every earlier event applied.
func (p *Projector) update(ctx context.Context, meta contracts.EventMeta, change func(*query.TodoView)) error {
	view, err := p.todos.Get(ctx, meta.TodoID)
	if err != nil {
		return fmt.Errorf("load todo view: %w", err)
	}
	change(view)
	view.UpdatedAt = meta.OccurredAt
	return p.todos.Upsert(ctx, view)
}
