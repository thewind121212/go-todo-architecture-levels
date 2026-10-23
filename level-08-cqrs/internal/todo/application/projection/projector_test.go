package projection_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/todo/application/projection"
	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
)

// fakeTodoViews and fakeActivityViews are in-memory read models. A fake, not a mock,
// because these tests check the views' final state, which is what idempotency is about.
// Embedding the interface satisfies it; the projector never calls List.
type fakeTodoViews struct {
	query.TodoViews
	views map[string]query.TodoView
}

func (f *fakeTodoViews) Get(_ context.Context, id string) (*query.TodoView, error) {
	v, ok := f.views[id]
	if !ok {
		return nil, query.ErrNotFound
	}
	return &v, nil
}

func (f *fakeTodoViews) Upsert(_ context.Context, v *query.TodoView) error {
	f.views[v.ID] = *v
	return nil
}

func (f *fakeTodoViews) Delete(_ context.Context, id string) error {
	delete(f.views, id)
	return nil
}

type fakeActivityViews struct {
	query.ActivityViews
	added []query.ActivityView // in insert order
}

func (f *fakeActivityViews) Add(_ context.Context, a *query.ActivityView) error {
	for _, existing := range f.added {
		if existing.ID == a.ID {
			return nil
		}
	}
	f.added = append(f.added, *a)
	return nil
}

var (
	t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 = t0.Add(time.Second)
)

type fixture struct {
	projector  *projection.Projector
	todos      *fakeTodoViews
	activities *fakeActivityViews
	todo       *domain.Todo
}

func newFixture(t *testing.T) *fixture {
	todos := &fakeTodoViews{views: map[string]query.TodoView{}}
	activities := &fakeActivityViews{}
	title, err := domain.NewTitle("buy milk")
	require.NoError(t, err)
	return &fixture{
		projector:  projection.NewProjector(todos, activities),
		todos:      todos,
		activities: activities,
		todo:       domain.NewTodo(domain.NewTodoID(), title, t0),
	}
}

// handleTwice projects every event the todo recorded since the last call, each one twice,
// as an at-least-once relay may do.
func (f *fixture) handleTwice(t *testing.T) {
	t.Helper()
	for _, e := range f.todo.PullEvents() {
		m, err := outbox.Encode(e)
		require.NoError(t, err)
		require.NoError(t, f.projector.Handle(context.Background(), m))
		require.NoError(t, f.projector.Handle(context.Background(), m), "a repeat must change nothing")
	}
}

func (f *fixture) view(t *testing.T) query.TodoView {
	t.Helper()
	v, ok := f.todos.views[f.todo.ID().String()]
	require.True(t, ok, "todo view missing")
	return v
}

func (f *fixture) actions() []string {
	var actions []string
	for _, a := range f.activities.added {
		actions = append(actions, a.Action)
	}
	return actions
}

func TestCreatedInsertsTheViewAndAnActivity(t *testing.T) {
	f := newFixture(t)

	f.handleTwice(t)

	id := f.todo.ID().String()
	require.Equal(t, query.TodoView{ID: id, Title: "buy milk", CreatedAt: t0, UpdatedAt: t0}, f.view(t))
	require.Len(t, f.activities.added, 1)
	created := f.activities.added[0]
	require.Equal(t, id, created.TodoID)
	require.Equal(t, "created", created.Action)
	require.Equal(t, t0, created.CreatedAt)
}

func TestRenamedChangesTheTitle(t *testing.T) {
	f := newFixture(t)
	f.handleTwice(t)
	title, err := domain.NewTitle("buy oat milk")
	require.NoError(t, err)

	f.todo.Rename(title, t1)
	f.handleTwice(t)

	require.Equal(t, "buy oat milk", f.view(t).Title)
	require.Equal(t, t1, f.view(t).UpdatedAt)
	require.Equal(t, t0, f.view(t).CreatedAt)
	require.Equal(t, []string{"created", "renamed"}, f.actions())
}

func TestCompletedAndReopenedFlipCompleted(t *testing.T) {
	f := newFixture(t)
	f.handleTwice(t)

	f.todo.Complete(t1)
	f.handleTwice(t)
	require.True(t, f.view(t).Completed)
	require.Equal(t, t1, f.view(t).UpdatedAt)

	f.todo.Reopen(t1.Add(time.Second))
	f.handleTwice(t)
	require.False(t, f.view(t).Completed)
	require.Equal(t, []string{"created", "completed", "reopened"}, f.actions())
}

func TestDeletedRemovesTheViewAndKeepsTheActivities(t *testing.T) {
	f := newFixture(t)
	f.handleTwice(t)

	f.todo.Delete(t1)
	f.handleTwice(t)

	require.Empty(t, f.todos.views)
	require.Equal(t, []string{"created", "deleted"}, f.actions())
}

// An update needs the view that the earlier events built. If it is missing, the event
// fails, and the relay retries it rather than skipping ahead.
func TestUpdateWithoutAViewFails(t *testing.T) {
	f := newFixture(t)
	f.todo.PullEvents() // the created event is never projected
	f.todo.Complete(t1)
	m, err := outbox.Encode(f.todo.PullEvents()[0])
	require.NoError(t, err)

	err = f.projector.Handle(context.Background(), m)

	require.ErrorIs(t, err, query.ErrNotFound)
	require.Empty(t, f.activities.added)
}
