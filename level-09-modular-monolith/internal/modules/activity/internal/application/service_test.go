package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/activity/internal/application"
	"example.com/todo/level09/internal/modules/activity/internal/application/mocks"
	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/platform/eventbus"
)

func newService(t *testing.T) (*application.Service, *mocks.MockActivityLog) {
	activities := mocks.NewMockActivityLog(t)
	return application.NewService(activities), activities
}

func TestRecordTurnsEveryTodoEventIntoAnActivity(t *testing.T) {
	meta := contracts.EventMeta{EventID: uuid.NewString(), TodoID: uuid.NewString(), OccurredAt: time.Now().UTC()}
	tests := []struct {
		event  eventbus.Event
		action string
	}{
		{contracts.TodoCreated{EventMeta: meta, Title: "buy milk"}, "created"},
		{contracts.TodoRenamed{EventMeta: meta, Title: "buy oat milk"}, "renamed"},
		{contracts.TodoCompleted{EventMeta: meta}, "completed"},
		{contracts.TodoReopened{EventMeta: meta}, "reopened"},
		{contracts.TodoDeleted{EventMeta: meta}, "deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			svc, activities := newService(t)
			want := &application.Activity{ID: meta.EventID, TodoID: meta.TodoID, Action: tt.action, CreatedAt: meta.OccurredAt}
			activities.EXPECT().Add(mock.Anything, want).Return(nil).Once()

			require.NoError(t, svc.Record(context.Background(), tt.event))
		})
	}
}

// The service subscribes to every todo event: publishing one on the bus records it.
func TestSubscribeListensToEveryTodoEvent(t *testing.T) {
	svc, activities := newService(t)
	bus := eventbus.New()
	svc.Subscribe(bus)
	meta := contracts.EventMeta{EventID: uuid.NewString(), TodoID: uuid.NewString(), OccurredAt: time.Now().UTC()}
	activities.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Times(5)

	for _, e := range []eventbus.Event{
		contracts.TodoCreated{EventMeta: meta}, contracts.TodoRenamed{EventMeta: meta},
		contracts.TodoCompleted{EventMeta: meta}, contracts.TodoReopened{EventMeta: meta},
		contracts.TodoDeleted{EventMeta: meta},
	} {
		require.NoError(t, bus.Publish(context.Background(), e))
	}
}

type otherEvent struct{}

func (otherEvent) EventName() string { return "todo.archived" }

func TestRecordRejectsAnUnknownEvent(t *testing.T) {
	svc, _ := newService(t)

	err := svc.Record(context.Background(), otherEvent{})

	require.ErrorContains(t, err, "unknown event")
}

// A failed insert is returned, so the relay does not mark the event and delivers it again.
func TestRecordReturnsAFailedInsert(t *testing.T) {
	svc, activities := newService(t)
	errBoom := errors.New("boom")
	activities.EXPECT().Add(mock.Anything, mock.Anything).Return(errBoom).Once()

	err := svc.Record(context.Background(), contracts.TodoCreated{})

	require.ErrorIs(t, err, errBoom)
}

func TestListActivitiesPassesTheCanonicalID(t *testing.T) {
	svc, activities := newService(t)
	id := uuid.NewString()
	activities.EXPECT().ListByTodo(mock.Anything, id).Return(nil, nil).Once()

	_, err := svc.ListActivities(context.Background(), strings.ToUpper(id))

	require.NoError(t, err)
}

// A malformed id names no todo: it is ErrNotFound, and the log is never asked.
func TestListActivitiesMalformedIDIsNotFound(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.ListActivities(context.Background(), "not-a-uuid")

	require.ErrorIs(t, err, application.ErrNotFound)
}
