// Package eventbus is an in-process, synchronous event bus: the one way modules talk to
// each other about what happened. A module publishes an event; every handler subscribed to
// that event's name runs, in the publisher's goroutine, before Publish returns.
//
// It is deliberately tiny. A broker (Kafka, NATS, RabbitMQ) would replace it once the
// modules become separate services; see the README's path to microservices.
package eventbus

import "context"

// Event is anything a module publishes. Its name is the subscription key.
type Event interface {
	EventName() string
}

// Handler reacts to one event. It runs in its own module and owns its transaction.
type Handler func(ctx context.Context, e Event) error

// Bus maps event names to their handlers.
//
// Subscribe every handler at startup, before the first Publish (Wire does it while it
// builds the modules). After that the bus is only read, so Publish is safe from any goroutine.
type Bus struct {
	handlers map[string][]Handler
}

func New() *Bus {
	return &Bus{handlers: map[string][]Handler{}}
}

// Subscribe adds h to the handlers of the event called name.
func (b *Bus) Subscribe(name string, h Handler) {
	b.handlers[name] = append(b.handlers[name], h)
}

// Publish calls the handlers of e's name in the order they subscribed, and stops at the
// first error. Handlers that already ran are not undone: each committed on its own. The
// caller (the outbox relay) retries the whole event, so every handler must be idempotent.
// An event nobody subscribed to is not an error.
func (b *Bus) Publish(ctx context.Context, e Event) error {
	for _, h := range b.handlers[e.EventName()] {
		if err := h(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
