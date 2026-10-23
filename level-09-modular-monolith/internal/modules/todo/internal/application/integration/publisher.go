// Package integration is where the todo module talks to the rest of the app: it turns the
// module's private domain events into its public integration events (package contracts) and
// publishes them on the event bus.
package integration

import (
	"context"
	"fmt"

	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/modules/todo/internal/codec"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/outbox"
)

// Publisher implements outbox.Handler: the relay hands it each message of the todo outbox,
// and it publishes the matching contract event. The bus runs every subscriber before Publish
// returns, so when Handle succeeds, every subscriber has committed and the relay may mark
// the message processed.
type Publisher struct {
	bus *eventbus.Bus
}

var _ outbox.Handler = (*Publisher)(nil)

func NewPublisher(bus *eventbus.Bus) *Publisher {
	return &Publisher{bus: bus}
}

// Handle decodes the stored domain event, translates it, and publishes it.
func (p *Publisher) Handle(ctx context.Context, m outbox.Message) error {
	event, err := codec.Decode(m)
	if err != nil {
		return err
	}
	published, err := toContract(event)
	if err != nil {
		return err
	}
	return p.bus.Publish(ctx, published)
}

// toContract is the one place where a domain event becomes an integration event. Today
// the two look alike; the point is that they can drift apart. The domain may rename a field,
// add an event or change a type, and only this function changes with it, not every
// subscriber in every other module.
func toContract(event domain.Event) (eventbus.Event, error) {
	m := event.Meta()
	meta := contracts.EventMeta{EventID: m.ID, TodoID: m.TodoID, OccurredAt: m.OccurredAt}
	switch e := event.(type) {
	case domain.TodoCreated:
		return contracts.TodoCreated{EventMeta: meta, Title: e.Title}, nil
	case domain.TodoRenamed:
		return contracts.TodoRenamed{EventMeta: meta, Title: e.Title}, nil
	case domain.TodoCompleted:
		return contracts.TodoCompleted{EventMeta: meta}, nil
	case domain.TodoReopened:
		return contracts.TodoReopened{EventMeta: meta}, nil
	case domain.TodoDeleted:
		return contracts.TodoDeleted{EventMeta: meta}, nil
	default:
		return nil, fmt.Errorf("publish: no contract for event %T", event)
	}
}
