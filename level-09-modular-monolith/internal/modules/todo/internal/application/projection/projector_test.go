package projection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/modules/todo/internal/application/projection"
	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/platform/eventbus"
)

// inTx marks a ctx that fakeTx handed out.
type inTx struct{}

// fakeTx stands in for the todo module's TxManager. The real transactions are tested
// against both databases in the contract suite.
type fakeTx struct{}

func (fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(context.WithValue(ctx, inTx{}, true))
}

var errNoTx = errors.New("write outside the projector's transaction")

// fakeTodoViews is an in-memory read model. A fake, not a mock, because these tests check
// the view's final state, which is what idempotency is about. It refuses writes outside a
// transaction. Embedding the interface satisfies it; the projector never calls List.
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

func (f *fakeTodoViews) Upsert(ctx context.Context, v *query.TodoView) error {
	if ctx.Value(inTx{}) == nil {
		return errNoTx
	}
	f.views[v.ID] = *v
	return nil
}

func (f *fakeTodoViews) Delete(ctx context.Context, id string) error {
	if ctx.Value(inTx{}) == nil {
		return errNoTx
	}
	delete(f.views, id)
	return nil
}

var (
	t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 = t0.Add(time.Second)
)

type fixture struct {
	projector *projection.Projector
	todos     *fakeTodoViews
	todoID    string
}

func newFixture() *fixture {
	todos := &fakeTodoViews{views: map[string]query.TodoView{}}
	return &fixture{projector: projection.NewProjector(todos, fakeTx{}), todos: todos, todoID: uuid.NewString()}
}

// meta is a new event about the fixture's todo.
func (f *fixture) meta(at time.Time) contracts.EventMeta {
	return contracts.EventMeta{EventID: uuid.NewString(), TodoID: f.todoID, OccurredAt: at}
}

// handleTwice projects each event twice, as an at-least-once relay may do.
func (f *fixture) handleTwice(t *testing.T, events ...eventbus.Event) {
	t.Helper()
	for _, e := range events {
		require.NoError(t, f.projector.Handle(context.Background(), e))
		require.NoError(t, f.projector.Handle(context.Background(), e), "a repeat must change nothing")
	}
}

func (f *fixture) view(t *testing.T) query.TodoView {
	t.Helper()
	v, ok := f.todos.views[f.todoID]
	require.True(t, ok, "todo view missing")
	return v
}

func (f *fixture) created(t *testing.T) {
	t.Helper()
	f.handleTwice(t, contracts.TodoCreated{EventMeta: f.meta(t0), Title: "buy milk"})
}

func TestCreatedInsertsTheView(t *testing.T) {
	f := newFixture()

	f.created(t)

	require.Equal(t, query.TodoView{ID: f.todoID, Title: "buy milk", CreatedAt: t0, UpdatedAt: t0}, f.view(t))
}

func TestRenamedChangesTheTitle(t *testing.T) {
	f := newFixture()
	f.created(t)

	f.handleTwice(t, contracts.TodoRenamed{EventMeta: f.meta(t1), Title: "buy oat milk"})

	require.Equal(t, "buy oat milk", f.view(t).Title)
	require.Equal(t, t1, f.view(t).UpdatedAt)
	require.Equal(t, t0, f.view(t).CreatedAt)
}

func TestCompletedAndReopenedFlipCompleted(t *testing.T) {
	f := newFixture()
	f.created(t)

	f.handleTwice(t, contracts.TodoCompleted{EventMeta: f.meta(t1)})
	require.True(t, f.view(t).Completed)
	require.Equal(t, t1, f.view(t).UpdatedAt)

	f.handleTwice(t, contracts.TodoReopened{EventMeta: f.meta(t1.Add(time.Second))})
	require.False(t, f.view(t).Completed)
}

func TestDeletedRemovesTheView(t *testing.T) {
	f := newFixture()
	f.created(t)

	f.handleTwice(t, contracts.TodoDeleted{EventMeta: f.meta(t1)})

	require.Empty(t, f.todos.views)
}

// An update needs the view that the earlier events built. If it is missing, the event
// fails, and the relay retries it rather than skipping ahead.
func TestUpdateWithoutAViewFails(t *testing.T) {
	f := newFixture()

	err := f.projector.Handle(context.Background(), contracts.TodoCompleted{EventMeta: f.meta(t1)})

	require.ErrorIs(t, err, query.ErrNotFound)
}

// The projector subscribes to every todo event on the bus.
func TestSubscribeListensToEveryTodoEvent(t *testing.T) {
	f := newFixture()
	bus := eventbus.New()
	f.projector.Subscribe(bus)
	ctx := context.Background()

	require.NoError(t, bus.Publish(ctx, contracts.TodoCreated{EventMeta: f.meta(t0), Title: "buy milk"}))
	require.NoError(t, bus.Publish(ctx, contracts.TodoRenamed{EventMeta: f.meta(t1), Title: "buy bread"}))
	require.Equal(t, "buy bread", f.view(t).Title)
	require.NoError(t, bus.Publish(ctx, contracts.TodoCompleted{EventMeta: f.meta(t1)}))
	require.True(t, f.view(t).Completed)
	require.NoError(t, bus.Publish(ctx, contracts.TodoReopened{EventMeta: f.meta(t1)}))
	require.False(t, f.view(t).Completed)
	require.NoError(t, bus.Publish(ctx, contracts.TodoDeleted{EventMeta: f.meta(t1)}))
	require.Empty(t, f.todos.views)
}
