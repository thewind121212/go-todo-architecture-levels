package codec_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/todo/internal/codec"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/outbox"
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
			m, err := codec.Encode(tt.event)
			require.NoError(t, err)
			require.Equal(t, tt.event.Meta().ID, m.ID)
			require.Equal(t, tt.event.Meta().TodoID, m.AggregateID)
			require.Equal(t, string(tt.event.Name()), m.Name)
			require.Equal(t, tt.event.Meta().OccurredAt, m.OccurredAt)
			require.JSONEq(t, tt.payload, string(m.Payload), "the payload is the event's own fields")

			got, err := codec.Decode(m)
			require.NoError(t, err)
			require.Equal(t, tt.event, got)
		})
	}
}

func TestDecodeRejectsUnknownMessages(t *testing.T) {
	_, err := codec.Decode(outbox.Message{ID: uuid.NewString(), Name: "archived", Payload: []byte(`{}`)})
	require.ErrorContains(t, err, `unknown name "archived"`)

	_, err = codec.Decode(outbox.Message{ID: uuid.NewString(), Name: "renamed", Payload: []byte(`{"title":`)})
	require.ErrorContains(t, err, "decode renamed event")
}
