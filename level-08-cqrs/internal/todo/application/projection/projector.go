// Package projection keeps the read models up to date. It is where the two sides of CQRS
// meet: it reads the write side's events from the outbox and writes the query side's views.
package projection

import (
	"context"
	"fmt"

	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
)

// Projector implements outbox.Handler: the relay hands it each event, inside the
// transaction that marks the event processed.
//
// Every write it makes is idempotent (upsert by id, delete, insert-if-absent), so applying
// an event twice leaves the views as applying it once. That is what makes the relay's
// at-least-once delivery safe.
type Projector struct {
	todos      query.TodoViews
	activities query.ActivityViews
}

var _ outbox.Handler = (*Projector)(nil)

func NewProjector(todos query.TodoViews, activities query.ActivityViews) *Projector {
	return &Projector{todos: todos, activities: activities}
}

// Handle applies one event to both read models.
func (p *Projector) Handle(ctx context.Context, m outbox.Message) error {
	event, err := outbox.Decode(m)
	if err != nil {
		return err
	}
	if err := p.projectTodo(ctx, event); err != nil {
		return err
	}
	// Every event is one line of the audit log, keyed by the event id.
	meta := event.Meta()
	return p.activities.Add(ctx, &query.ActivityView{
		ID:        meta.ID,
		TodoID:    meta.TodoID,
		Action:    string(event.Name()),
		CreatedAt: meta.OccurredAt,
	})
}

// projectTodo turns the event into the todo view's new state.
func (p *Projector) projectTodo(ctx context.Context, event domain.Event) error {
	meta := event.Meta()
	switch e := event.(type) {
	case domain.TodoCreated:
		return p.todos.Upsert(ctx, &query.TodoView{
			ID:        meta.TodoID,
			Title:     e.Title,
			CreatedAt: meta.OccurredAt,
			UpdatedAt: meta.OccurredAt,
		})
	case domain.TodoRenamed:
		return p.update(ctx, meta, func(v *query.TodoView) { v.Title = e.Title })
	case domain.TodoCompleted:
		return p.update(ctx, meta, func(v *query.TodoView) { v.Completed = true })
	case domain.TodoReopened:
		return p.update(ctx, meta, func(v *query.TodoView) { v.Completed = false })
	case domain.TodoDeleted:
		return p.todos.Delete(ctx, meta.TodoID)
	default:
		return fmt.Errorf("project todo: unknown event %T", event)
	}
}

// update loads the view, changes it, and upserts it with the event's time as updated_at.
// Events arrive in order, so the view it loads already has every earlier event applied.
func (p *Projector) update(ctx context.Context, meta domain.EventMeta, change func(*query.TodoView)) error {
	view, err := p.todos.Get(ctx, meta.TodoID)
	if err != nil {
		return fmt.Errorf("load todo view: %w", err)
	}
	change(view)
	view.UpdatedAt = meta.OccurredAt
	return p.todos.Upsert(ctx, view)
}
