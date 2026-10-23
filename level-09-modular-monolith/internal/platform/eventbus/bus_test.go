package eventbus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/platform/eventbus"
)

type event string

func (e event) EventName() string { return string(e) }

// record returns a handler that appends its name to calls, then returns err.
func record(calls *[]string, name string, err error) eventbus.Handler {
	return func(_ context.Context, e eventbus.Event) error {
		*calls = append(*calls, name+" got "+e.EventName())
		return err
	}
}

func TestPublishCallsTheSubscribersOfTheNameInOrder(t *testing.T) {
	bus := eventbus.New()
	var calls []string
	bus.Subscribe("todo.created", record(&calls, "first", nil))
	bus.Subscribe("todo.deleted", record(&calls, "other", nil))
	bus.Subscribe("todo.created", record(&calls, "second", nil))

	require.NoError(t, bus.Publish(context.Background(), event("todo.created")))

	require.Equal(t, []string{"first got todo.created", "second got todo.created"}, calls)
}

func TestPublishWithoutSubscribersIsNotAnError(t *testing.T) {
	require.NoError(t, eventbus.New().Publish(context.Background(), event("todo.created")))
}

// The first failure stops the delivery and is returned as is, so the caller can retry the event.
func TestPublishStopsAtTheFirstError(t *testing.T) {
	bus := eventbus.New()
	errBoom := errors.New("boom")
	var calls []string
	bus.Subscribe("todo.created", record(&calls, "first", nil))
	bus.Subscribe("todo.created", record(&calls, "failing", errBoom))
	bus.Subscribe("todo.created", record(&calls, "never", nil))

	err := bus.Publish(context.Background(), event("todo.created"))

	require.ErrorIs(t, err, errBoom)
	require.Equal(t, []string{"first got todo.created", "failing got todo.created"}, calls)
}
