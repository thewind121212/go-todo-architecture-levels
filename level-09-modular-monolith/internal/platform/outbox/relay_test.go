package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/outbox"
	"example.com/todo/level09/internal/platform/outbox/mocks"
)

func newRelay(t *testing.T) (*outbox.Relay, *mocks.MockStore, *mocks.MockHandler) {
	store, handler := mocks.NewMockStore(t), mocks.NewMockHandler(t)
	return outbox.NewRelay(store, handler, time.Millisecond, zerolog.Nop()), store, handler
}

func TestPollOnceDeliversThenMarksEachMessageInOrder(t *testing.T) {
	relay, store, handler := newRelay(t)
	first, second := outbox.Message{ID: "1", Name: "created"}, outbox.Message{ID: "2", Name: "renamed"}
	store.EXPECT().Unprocessed(mock.Anything, mock.Anything).Return([]outbox.Message{first, second}, nil).Once()
	var calls []string
	handler.EXPECT().Handle(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, m outbox.Message) error {
		calls = append(calls, "handle "+m.ID)
		return nil
	}).Twice()
	store.EXPECT().MarkProcessed(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, id string, _ time.Time) error {
		calls = append(calls, "mark "+id)
		return nil
	}).Twice()

	require.NoError(t, relay.PollOnce(context.Background()))
	require.Equal(t, []string{"handle 1", "mark 1", "handle 2", "mark 2"}, calls)
}

// A failed message is not marked, and nothing after it runs: the next poll retries it
// first, so events are never applied out of order.
func TestPollOnceStopsAtTheFirstFailure(t *testing.T) {
	relay, store, handler := newRelay(t)
	errBoom := errors.New("boom")
	first, second := outbox.Message{ID: "1", Name: "created"}, outbox.Message{ID: "2", Name: "renamed"}
	store.EXPECT().Unprocessed(mock.Anything, mock.Anything).Return([]outbox.Message{first, second}, nil).Once()
	handler.EXPECT().Handle(mock.Anything, first).Return(errBoom).Once()

	err := relay.PollOnce(context.Background())

	require.ErrorIs(t, err, errBoom)
	require.ErrorContains(t, err, "handle created event 1")
}

// published is a message as an event on the bus.
type published struct{ outbox.Message }

func (p published) EventName() string { return p.Name }

// publisher is the Handler a module plugs in: it publishes each message on the bus.
type publisher struct{ bus *eventbus.Bus }

func (p publisher) Handle(ctx context.Context, m outbox.Message) error {
	return p.bus.Publish(ctx, published{m})
}

// Deliver-then-mark with two subscribers: the first commits, the second fails, so the
// message is not marked. The next poll delivers it to both again. The first one sees an
// event it already applied and changes nothing (it is idempotent); the second catches up;
// only then is the message marked.
func TestAFailedSubscriberGetsTheMessageAgain(t *testing.T) {
	store := mocks.NewMockStore(t)
	bus := eventbus.New()
	m := outbox.Message{ID: "1", Name: "created"}
	store.EXPECT().Unprocessed(mock.Anything, mock.Anything).Return([]outbox.Message{m}, nil).Twice()
	store.EXPECT().MarkProcessed(mock.Anything, "1", mock.Anything).Return(nil).Once()

	applied := map[string]bool{} // the first subscriber's state: event ids it has applied
	deliveries := 0
	bus.Subscribe("created", func(_ context.Context, e eventbus.Event) error {
		deliveries++
		applied[e.(published).ID] = true // an upsert by event id: a repeat is a no-op
		return nil
	})
	failures := 1
	bus.Subscribe("created", func(context.Context, eventbus.Event) error {
		if failures > 0 {
			failures--
			return errors.New("activity store down")
		}
		return nil
	})
	relay := outbox.NewRelay(store, publisher{bus}, time.Millisecond, zerolog.Nop())

	require.ErrorContains(t, relay.PollOnce(context.Background()), "activity store down")
	require.NoError(t, relay.PollOnce(context.Background()))

	require.Equal(t, 2, deliveries, "redelivered to the subscriber that had succeeded")
	require.Equal(t, map[string]bool{"1": true}, applied, "and the repeat changed nothing")
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	relay, store, _ := newRelay(t)
	store.EXPECT().Unprocessed(mock.Anything, mock.Anything).Return(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		relay.Run(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
