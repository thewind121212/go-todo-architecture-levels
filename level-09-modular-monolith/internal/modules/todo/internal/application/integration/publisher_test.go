package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/modules/todo/internal/application/integration"
	"example.com/todo/level09/internal/modules/todo/internal/codec"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/outbox"
)

// subscribeAll returns a bus on which every todo event is appended to got.
func subscribeAll(got *[]eventbus.Event) *eventbus.Bus {
	bus := eventbus.New()
	for _, name := range []string{
		contracts.TodoCreatedName, contracts.TodoRenamedName, contracts.TodoCompletedName,
		contracts.TodoReopenedName, contracts.TodoDeletedName,
	} {
		bus.Subscribe(name, func(_ context.Context, e eventbus.Event) error {
			*got = append(*got, e)
			return nil
		})
	}
	return bus
}

// Every domain event, read back from the outbox, is published as its contract event,
// with the same id, todo id, time and data.
func TestHandlePublishesTheContractOfEveryEvent(t *testing.T) {
	m := domain.EventMeta{ID: uuid.NewString(), TodoID: uuid.NewString(), OccurredAt: time.Now().UTC()}
	meta := contracts.EventMeta{EventID: m.ID, TodoID: m.TodoID, OccurredAt: m.OccurredAt}
	tests := []struct {
		event domain.Event
		want  eventbus.Event
	}{
		{domain.TodoCreated{EventMeta: m, Title: "buy milk"}, contracts.TodoCreated{EventMeta: meta, Title: "buy milk"}},
		{domain.TodoRenamed{EventMeta: m, Title: "buy oat milk"}, contracts.TodoRenamed{EventMeta: meta, Title: "buy oat milk"}},
		{domain.TodoCompleted{EventMeta: m}, contracts.TodoCompleted{EventMeta: meta}},
		{domain.TodoReopened{EventMeta: m}, contracts.TodoReopened{EventMeta: meta}},
		{domain.TodoDeleted{EventMeta: m}, contracts.TodoDeleted{EventMeta: meta}},
	}
	for _, tt := range tests {
		t.Run(string(tt.event.Name()), func(t *testing.T) {
			var got []eventbus.Event
			publisher := integration.NewPublisher(subscribeAll(&got))
			message, err := codec.Encode(tt.event)
			require.NoError(t, err)

			require.NoError(t, publisher.Handle(context.Background(), message))

			require.Equal(t, []eventbus.Event{tt.want}, got)
		})
	}
}

// A message that cannot be decoded is an error, and nothing is published.
func TestHandleRejectsAnUnknownMessage(t *testing.T) {
	var got []eventbus.Event
	publisher := integration.NewPublisher(subscribeAll(&got))

	err := publisher.Handle(context.Background(), outbox.Message{ID: uuid.NewString(), Name: "archived"})

	require.ErrorContains(t, err, `unknown name "archived"`)
	require.Empty(t, got)
}
