package outbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/todo/outbox"
	"example.com/todo/level08/internal/todo/outbox/mocks"
)

// noTx runs fn directly. These tests are about order and marking; the real
// transactions are tested against both databases in the contract suite.
type noTx struct{}

func (noTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newRelay(t *testing.T) (*outbox.Relay, *mocks.MockStore, *mocks.MockHandler) {
	store, handler := mocks.NewMockStore(t), mocks.NewMockHandler(t)
	return outbox.NewRelay(store, noTx{}, handler, time.Millisecond, zerolog.Nop()), store, handler
}

func TestPollOnceHandlesThenMarksEachMessageInOrder(t *testing.T) {
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
