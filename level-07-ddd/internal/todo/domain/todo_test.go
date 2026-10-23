package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level07/internal/todo/domain"
)

var (
	now   = time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)
	later = now.Add(time.Second)
)

func title(t *testing.T, raw string) domain.Title {
	t.Helper()
	title, err := domain.NewTitle(raw)
	require.NoError(t, err)
	return title
}

// stored is a todo as a repository rebuilds it: version 3, no pending events.
func stored(t *testing.T, completed bool) *domain.Todo {
	t.Helper()
	return domain.RehydrateTodo(uuid.NewString(), "old", completed, now, now, 3)
}

func names(events []domain.Event) []domain.Action {
	var names []domain.Action
	for _, e := range events {
		names = append(names, e.Name())
	}
	return names
}

func TestNewTodoRecordsCreated(t *testing.T) {
	id := domain.NewTodoID()

	todo := domain.NewTodo(id, title(t, "buy milk"), now)

	require.Equal(t, id, todo.ID())
	require.Equal(t, "buy milk", todo.Title().String())
	require.False(t, todo.Completed())
	require.Equal(t, now, todo.CreatedAt())
	require.Equal(t, now, todo.UpdatedAt())
	require.Equal(t, 0, todo.Version(), "never saved")

	events := todo.PullEvents()
	require.Len(t, events, 1)
	created, ok := events[0].(domain.TodoCreated)
	require.True(t, ok)
	require.Equal(t, domain.ActionCreated, created.Name())
	require.Equal(t, "buy milk", created.Title)
	require.Equal(t, id.String(), created.TodoID)
	require.Equal(t, now, created.OccurredAt)
	require.Equal(t, uuid.Version(7), uuid.MustParse(created.ID).Version())
}

func TestRehydrateTodoRecordsNothing(t *testing.T) {
	todo := stored(t, true)

	require.True(t, todo.Completed())
	require.Equal(t, 3, todo.Version())
	require.Empty(t, todo.PullEvents())
}

func TestRename(t *testing.T) {
	todo := stored(t, false)

	todo.Rename(title(t, " old "), later)
	require.Empty(t, todo.PullEvents(), "same title after trimming is not a change")
	require.Equal(t, now, todo.UpdatedAt())

	todo.Rename(title(t, "new"), later)
	require.Equal(t, "new", todo.Title().String())
	require.Equal(t, later, todo.UpdatedAt())
	events := todo.PullEvents()
	require.Len(t, events, 1)
	renamed, ok := events[0].(domain.TodoRenamed)
	require.True(t, ok)
	require.Equal(t, "new", renamed.Title, "the event carries the new title")
	require.Equal(t, later, renamed.OccurredAt)
}

func TestCompleteAndReopen(t *testing.T) {
	todo := stored(t, false)

	todo.Reopen(later)
	require.Empty(t, todo.PullEvents(), "reopening an open todo is not a change")

	todo.Complete(later)
	todo.Complete(later)
	require.True(t, todo.Completed())
	require.Equal(t, []domain.Action{domain.ActionCompleted}, names(todo.PullEvents()), "only the first Complete changes anything")

	todo.Reopen(later)
	require.False(t, todo.Completed())
	require.Equal(t, []domain.Action{domain.ActionReopened}, names(todo.PullEvents()))
}

func TestDelete(t *testing.T) {
	todo := stored(t, false)

	todo.Delete(later)
	todo.Delete(later)

	require.True(t, todo.Deleted())
	events := todo.PullEvents()
	require.Equal(t, []domain.Action{domain.ActionDeleted}, names(events), "deleting twice is one change")
	require.Equal(t, todo.ID().String(), events[0].Meta().TodoID)
}

// Version is the loaded version: behavior records events but leaves it to the repository.
func TestBehaviorKeepsTheLoadedVersion(t *testing.T) {
	todo := stored(t, false)

	todo.Rename(title(t, "new"), later)
	todo.Complete(later)
	todo.Delete(later)

	require.Equal(t, 3, todo.Version())
	require.Equal(t, []domain.Action{domain.ActionRenamed, domain.ActionCompleted, domain.ActionDeleted}, names(todo.PullEvents()))
	require.Equal(t, 3, todo.Version())
}

func TestPullEventsClears(t *testing.T) {
	todo := domain.NewTodo(domain.NewTodoID(), title(t, "task"), now)

	require.Len(t, todo.PullEvents(), 1)
	require.Empty(t, todo.PullEvents())
}
