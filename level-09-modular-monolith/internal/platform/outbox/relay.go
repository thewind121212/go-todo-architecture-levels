package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// batchSize caps how many messages one poll processes; the rest wait for the next poll.
const batchSize = 100

// Store is the outbox table or collection, as the relay sees it.
type Store interface {
	// Unprocessed returns up to limit messages that are not processed yet, in id order.
	Unprocessed(ctx context.Context, limit int) ([]Message, error)
	MarkProcessed(ctx context.Context, id string, at time.Time) error
}

// Handler delivers one message, here by publishing it on the event bus. Delivery is
// at-least-once (a message can be handed over again), so whatever receives it must be idempotent.
type Handler interface {
	Handle(ctx context.Context, m Message) error
}

// Relay drains an outbox: it polls for unprocessed messages and passes each one to the
// handler. It is the only reader of the outbox, so it needs no locks (see the README for
// running several relays).
type Relay struct {
	store    Store
	handler  Handler
	interval time.Duration
	log      zerolog.Logger
}

func NewRelay(store Store, handler Handler, interval time.Duration, log zerolog.Logger) *Relay {
	return &Relay{store: store, handler: handler, interval: interval, log: log}
}

// Run polls every interval until ctx is cancelled. A failed poll is logged and retried on
// the next tick; its message stays unprocessed, so nothing is lost.
func (r *Relay) Run(ctx context.Context) {
	r.log.Info().Stringer("interval", r.interval).Msg("relay started")
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.PollOnce(ctx); err != nil && ctx.Err() == nil {
			r.log.Error().Err(err).Msg("relay poll failed")
		}
		select {
		case <-ctx.Done():
			r.log.Info().Msg("relay stopped")
			return
		case <-ticker.C:
		}
	}
}

// PollOnce processes the oldest unprocessed messages, up to batchSize, in id order.
// It stops at the first failure, so a later event never overtakes an earlier one.
func (r *Relay) PollOnce(ctx context.Context) error {
	messages, err := r.store.Unprocessed(ctx, batchSize)
	if err != nil {
		return err
	}
	for _, m := range messages {
		if err := r.process(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// process delivers the message, then marks it processed. There is no transaction around
// the two: each subscriber commits in its own module's transaction, which the relay neither
// sees nor joins. If delivery fails, or the mark fails after it, the message stays
// unprocessed and is delivered again on the next poll, also to the subscribers that
// already handled it. That is at-least-once delivery, and why every subscriber is idempotent.
func (r *Relay) process(ctx context.Context, m Message) error {
	if err := r.handler.Handle(ctx, m); err != nil {
		return fmt.Errorf("handle %s event %s: %w", m.Name, m.ID, err)
	}
	return r.store.MarkProcessed(ctx, m.ID, time.Now().UTC())
}
