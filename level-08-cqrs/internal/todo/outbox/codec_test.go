package outbox_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
)

func meta() domain.EventMeta {
	return domain.EventMeta{ID: uuid.NewString(), TodoID: uuid.NewString(), OccurredAt: time.Now().UTC()}
}

func TestEncodeDecodeRoundTripsEveryEvent(t *testing.T) {
	tests := []struct {
		event   domain.Event
		payload string
	}{
		{domain.TodoCreated{EventMeta: meta(), Title: "buy milk"}, `{"title":"buy milk"}`},
		{domain.TodoRenamed{EventMeta: meta(), Title: "buy oat milk"}, `{"title":"buy oat milk"}`},
		{domain.TodoCompleted{EventMeta: meta()}, `{}`},
		{domain.TodoReopened{EventMeta: meta()}, `{}`},
		{domain.TodoDeleted{EventMeta: meta()}, `{}`},
	}
	for _, tt := range tests {
		t.Run(string(tt.event.Name()), func(t *testing.T) {
			m, err := outbox.Encode(tt.event)
			require.NoError(t, err)
			require.Equal(t, tt.event.Meta().ID, m.ID)
			require.Equal(t, tt.event.Meta().TodoID, m.TodoID)
			require.Equal(t, string(tt.event.Name()), m.Name)
			require.Equal(t, tt.event.Meta().OccurredAt, m.OccurredAt)
			require.JSONEq(t, tt.payload, string(m.Payload), "the payload is the event's own fields")

			got, err := outbox.Decode(m)
			require.NoError(t, err)
			require.Equal(t, tt.event, got)
		})
	}
}

func TestDecodeRejectsUnknownMessages(t *testing.T) {
	_, err := outbox.Decode(outbox.Message{ID: uuid.NewString(), Name: "archived", Payload: []byte(`{}`)})
	require.ErrorContains(t, err, `unknown name "archived"`)

	_, err = outbox.Decode(outbox.Message{ID: uuid.NewString(), Name: "renamed", Payload: []byte(`{"title":`)})
	require.ErrorContains(t, err, "decode renamed event")
}
