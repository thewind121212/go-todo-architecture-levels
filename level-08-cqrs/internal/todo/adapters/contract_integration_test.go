//go:build integration

// Package adapters_test runs ONE contract suite against BOTH driven adapters,
// so you can see that Postgres and Mongo honor the repository and the ports identically.
package adapters_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/v2/bson"

	"example.com/todo/level08/internal/infrastructure/database"
	mongoadapter "example.com/todo/level08/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level08/internal/todo/adapters/postgres"
	"example.com/todo/level08/internal/todo/application/projection"
	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
)

// adapters is one storage choice, seen only through the repository and the ports.
type adapters struct {
	todos      domain.TodoRepository
	outbox     outbox.Store
	tx         outbox.TxManager
	todoViews  query.TodoViews
	activities query.ActivityViews
	// rejectOutbox makes the database refuse outbox entries for one todo until the test
	// ends. It is the only storage-specific hook: it lets the suite break Save halfway.
	rejectOutbox func(t *testing.T, id domain.TodoID)
}

func TestPostgresAdapters(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("todo"),
		tcpostgres.WithUsername("todo"),
		tcpostgres.WithPassword("todo"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// OpenPostgres applies the real embedded SQL migrations, like the app does.
	db, closeDB, err := database.OpenPostgres(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(closeDB)

	runContract(t, adapters{
		todos:      pgadapter.NewTodoRepository(db),
		outbox:     pgadapter.NewOutbox(db),
		tx:         pgadapter.NewTxManager(db),
		todoViews:  pgadapter.NewTodoViews(db),
		activities: pgadapter.NewActivityViews(db),
		rejectOutbox: func(t *testing.T, id domain.TodoID) {
			// DDL takes no bind parameters; id is a parsed UUID, so formatting it in is safe.
			check := fmt.Sprintf("ALTER TABLE outbox ADD CONSTRAINT reject_todo CHECK (todo_id <> '%s')", id)
			require.NoError(t, db.Exec(check).Error)
			t.Cleanup(func() { require.NoError(t, db.Exec("ALTER TABLE outbox DROP CONSTRAINT reject_todo").Error) })
		},
	})
}

func TestMongoAdapters(t *testing.T) {
	ctx := context.Background()
	// Transactions need a replica set, even a single-node one.
	ctr, err := mongodb.Run(ctx, "mongo:8", mongodb.WithReplicaSet("rs0"))
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)
	uri, err := ctr.ConnectionString(ctx)
	require.NoError(t, err)

	// directConnection: the replica-set member's advertised host is not reachable from here.
	db, disconnect, err := database.OpenMongo(ctx, uri+"&directConnection=true", "todo_test")
	require.NoError(t, err)
	t.Cleanup(disconnect)

	runContract(t, adapters{
		todos:      mongoadapter.NewTodoRepository(db),
		outbox:     mongoadapter.NewOutbox(db),
		tx:         mongoadapter.NewTxManager(db),
		todoViews:  mongoadapter.NewTodoViews(db),
		activities: mongoadapter.NewActivityViews(db),
		rejectOutbox: func(t *testing.T, id domain.TodoID) {
			validator := func(rule bson.D) bson.D {
				return bson.D{{Key: "collMod", Value: "outbox"}, {Key: "validator", Value: rule}}
			}
			reject := bson.D{{Key: "todo_id", Value: bson.D{{Key: "$ne", Value: id.String()}}}}
			require.NoError(t, db.RunCommand(ctx, validator(reject)).Err())
			t.Cleanup(func() { require.NoError(t, db.RunCommand(ctx, validator(bson.D{})).Err()) })
		},
	})
}

// runContract is the behavior every adapter must have. Subtests share one
// database and run in order.
func runContract(t *testing.T, a adapters) {
	ctx := context.Background()

	// load reads a todo back, the way every command starts.
	load := func(t *testing.T, id domain.TodoID) *domain.Todo {
		t.Helper()
		todo, err := a.todos.Get(ctx, id)
		require.NoError(t, err)
		return todo
	}
	// pending returns the todo's unprocessed outbox messages, oldest first.
	pending := func(t *testing.T, id domain.TodoID) []outbox.Message {
		t.Helper()
		all, err := a.outbox.Unprocessed(ctx, 1000)
		require.NoError(t, err)
		var mine []outbox.Message
		for _, m := range all {
			if m.TodoID == id.String() {
				mine = append(mine, m)
			}
		}
		return mine
	}
	pendingNames := func(t *testing.T, id domain.TodoID) []string {
		t.Helper()
		var names []string
		for _, m := range pending(t, id) {
			names = append(names, m.Name)
		}
		return names
	}
	actions := func(t *testing.T, id domain.TodoID) []string {
		t.Helper()
		activities, err := a.activities.ListByTodo(ctx, id.String())
		require.NoError(t, err)
		var actions []string
		for _, act := range activities {
			actions = append(actions, act.Action)
		}
		return actions
	}
	// saveNew stores a new todo and returns it as stored (a Todo serves one command).
	saveNew := func(t *testing.T, title string) *domain.Todo {
		t.Helper()
		todo := newTodo(t, title)
		require.NoError(t, a.todos.Save(ctx, todo))
		return load(t, todo.ID())
	}
	// newRelay is the app's relay over this storage, with the real projector unless
	// the test passes its own handler.
	projector := projection.NewProjector(a.todoViews, a.activities)
	newRelay := func(handler outbox.Handler) *outbox.Relay {
		return outbox.NewRelay(a.outbox, a.tx, handler, time.Second, zerolog.Nop())
	}

	// ---- Write side: the aggregate and the outbox ----

	t.Run("save then get round-trips every field and writes the event to the outbox", func(t *testing.T) {
		todo := newTodo(t, "round trip")
		require.NoError(t, a.todos.Save(ctx, todo))

		got := load(t, todo.ID())
		require.Equal(t, 1, got.Version(), "the first save stores version 1")
		require.Equal(t, todo.ID(), got.ID())
		require.Equal(t, todo.Title(), got.Title())
		require.Equal(t, todo.Completed(), got.Completed())
		require.Equal(t, todo.CreatedAt(), got.CreatedAt())
		require.Equal(t, todo.UpdatedAt(), got.UpdatedAt())

		messages := pending(t, todo.ID())
		require.Len(t, messages, 1)
		require.Equal(t, uuid.Version(7), uuid.MustParse(messages[0].ID).Version(), "the event's id")
		require.Equal(t, todo.CreatedAt(), messages[0].OccurredAt, "the event's time")
		event, err := outbox.Decode(messages[0])
		require.NoError(t, err)
		created, ok := event.(domain.TodoCreated)
		require.True(t, ok)
		require.Equal(t, "round trip", created.Title, "the payload survives the database")
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		_, err := a.todos.Get(ctx, domain.NewTodoID())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("save persists each change, appends its events in order and bumps the version", func(t *testing.T) {
		id := saveNew(t, "before").ID()

		todo := load(t, id)
		todo.Rename(title(t, "after"), now())
		todo.Complete(now())
		require.NoError(t, a.todos.Save(ctx, todo))
		got := load(t, id)
		require.Equal(t, "after", got.Title().String())
		require.True(t, got.Completed())
		require.Equal(t, todo.UpdatedAt(), got.UpdatedAt())
		require.Equal(t, 2, got.Version(), "two events, one save, one new version")

		todo = load(t, id)
		todo.Reopen(now())
		require.NoError(t, a.todos.Save(ctx, todo))
		got = load(t, id)
		require.False(t, got.Completed())
		require.Equal(t, 3, got.Version())

		require.Equal(t, []string{"created", "renamed", "completed", "reopened"}, pendingNames(t, id))
	})

	t.Run("save without pending events writes nothing", func(t *testing.T) {
		id := saveNew(t, "unchanged").ID()

		todo := load(t, id)
		todo.Rename(title(t, "unchanged"), now())
		require.NoError(t, a.todos.Save(ctx, todo))

		require.Equal(t, 1, load(t, id).Version())
		require.Equal(t, []string{"created"}, pendingNames(t, id))
	})

	t.Run("delete removes the todo and appends deleted to the outbox", func(t *testing.T) {
		id := saveNew(t, "doomed").ID()

		todo := load(t, id)
		todo.Delete(now())
		require.NoError(t, a.todos.Save(ctx, todo))

		_, err := a.todos.Get(ctx, id)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, []string{"created", "deleted"}, pendingNames(t, id))
	})

	// Optimistic locking: a save only succeeds on the version it loaded.
	t.Run("a stale save is ErrConflict and writes no outbox entry", func(t *testing.T) {
		id := saveNew(t, "contended").ID()
		first, second, third := load(t, id), load(t, id), load(t, id)

		first.Rename(title(t, "first wins"), now())
		require.NoError(t, a.todos.Save(ctx, first))

		second.Complete(now())
		require.ErrorIs(t, a.todos.Save(ctx, second), domain.ErrConflict)
		third.Delete(now())
		require.ErrorIs(t, a.todos.Save(ctx, third), domain.ErrConflict)

		got := load(t, id)
		require.Equal(t, "first wins", got.Title().String())
		require.False(t, got.Completed())
		require.Equal(t, 2, got.Version())
		require.Equal(t, []string{"created", "renamed"}, pendingNames(t, id))
	})

	// The aggregate is the transaction boundary: if its events cannot be stored,
	// its state is not stored either. No dual write, no lost event.
	t.Run("save rolls the state back when the outbox insert fails", func(t *testing.T) {
		todo := newTodo(t, "rolled back")
		a.rejectOutbox(t, todo.ID())

		// The todo insert succeeded; the outbox insert after it failed.
		require.ErrorContains(t, a.todos.Save(ctx, todo), "insert outbox")

		_, err := a.todos.Get(ctx, todo.ID())
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Empty(t, pending(t, todo.ID()))
	})

	// ---- Read side: the read models ----

	t.Run("todo views upsert, get, list newest first and delete", func(t *testing.T) {
		first, second, third := view("v1"), view("v2"), view("v3")
		for _, v := range []*query.TodoView{first, second, third} {
			require.NoError(t, a.todoViews.Upsert(ctx, v))
		}

		changed := *second
		changed.Title, changed.Completed, changed.UpdatedAt = "v2 done", true, now().Add(time.Minute)
		require.NoError(t, a.todoViews.Upsert(ctx, &changed))
		require.NoError(t, a.todoViews.Upsert(ctx, &changed), "upserting twice is one change")
		got, err := a.todoViews.Get(ctx, second.ID)
		require.NoError(t, err)
		require.Equal(t, &changed, got)

		page, err := a.todoViews.List(ctx, 2, 0)
		require.NoError(t, err)
		require.Equal(t, []*query.TodoView{third, &changed}, page)
		page, err = a.todoViews.List(ctx, 10, 1000)
		require.NoError(t, err)
		require.NotNil(t, page)
		require.Empty(t, page)

		require.NoError(t, a.todoViews.Delete(ctx, first.ID))
		require.NoError(t, a.todoViews.Delete(ctx, first.ID), "deleting twice is not an error")
		_, err = a.todoViews.Get(ctx, first.ID)
		require.ErrorIs(t, err, query.ErrNotFound)
	})

	t.Run("activity views ignore a repeated id and list oldest first, per todo", func(t *testing.T) {
		todoID := uuid.NewString()
		first := &query.ActivityView{ID: newID(), TodoID: todoID, Action: "created", CreatedAt: now()}
		second := &query.ActivityView{ID: newID(), TodoID: todoID, Action: "renamed", CreatedAt: now()}
		other := &query.ActivityView{ID: newID(), TodoID: uuid.NewString(), Action: "created", CreatedAt: now()}
		for _, act := range []*query.ActivityView{first, second, other} {
			require.NoError(t, a.activities.Add(ctx, act))
		}
		repeat := *first
		repeat.Action = "renamed"
		require.NoError(t, a.activities.Add(ctx, &repeat), "a repeated id is not an error")

		got, err := a.activities.ListByTodo(ctx, todoID)
		require.NoError(t, err)
		require.Equal(t, []*query.ActivityView{first, second}, got, "the first write wins")

		none, err := a.activities.ListByTodo(ctx, uuid.NewString())
		require.NoError(t, err)
		require.NotNil(t, none)
		require.Empty(t, none)
	})

	// ---- The relay: outbox -> projector -> read models ----

	t.Run("mark processed removes a message from the unprocessed list", func(t *testing.T) {
		id := saveNew(t, "marked").ID()
		m := pending(t, id)[0]

		require.NoError(t, a.outbox.MarkProcessed(ctx, m.ID, now()))

		require.Empty(t, pending(t, id))
	})

	t.Run("relay projects in id order, marks processed, and skips processed events", func(t *testing.T) {
		todo := saveNew(t, "relayed")
		todo.Rename(title(t, "relayed v2"), now())
		todo.Complete(now())
		require.NoError(t, a.todos.Save(ctx, todo))
		rec := &recorder{next: projector}

		require.NoError(t, newRelay(rec).PollOnce(ctx))

		require.NotEmpty(t, rec.ids)
		require.True(t, slices.IsSorted(rec.ids), "oldest event first")
		require.Empty(t, pending(t, todo.ID()))
		got, err := a.todoViews.Get(ctx, todo.ID().String())
		require.NoError(t, err)
		require.Equal(t, &query.TodoView{
			ID:        todo.ID().String(),
			Title:     "relayed v2",
			Completed: true,
			CreatedAt: todo.CreatedAt(),
			UpdatedAt: todo.UpdatedAt(),
		}, got)
		require.Equal(t, []string{"created", "renamed", "completed"}, actions(t, todo.ID()))

		again := &recorder{next: projector}
		require.NoError(t, newRelay(again).PollOnce(ctx))
		require.Empty(t, again.ids, "processed events are not handed over again")
	})

	t.Run("a failed projection leaves the event unprocessed and the views untouched", func(t *testing.T) {
		id := saveNew(t, "failing").ID()
		failing := &recorder{next: projector, err: errors.New("boom")}

		require.ErrorContains(t, newRelay(failing).PollOnce(ctx), "boom")

		require.Equal(t, []string{"created"}, pendingNames(t, id), "retried on the next poll")
		_, err := a.todoViews.Get(ctx, id.String())
		require.ErrorIs(t, err, query.ErrNotFound, "the projector's upsert was rolled back")
		require.Empty(t, actions(t, id))
	})

	// At-least-once delivery: an event can be applied and then fail to be marked, here by
	// projecting it outside the relay. Handing it over again must change nothing.
	t.Run("an event applied but not marked is applied again harmlessly", func(t *testing.T) {
		todo := saveNew(t, "crashed")
		todo.Rename(title(t, "crashed v2"), now())
		require.NoError(t, a.todos.Save(ctx, todo))
		for _, m := range pending(t, todo.ID()) {
			require.NoError(t, projector.Handle(ctx, m)) // applied, then "crash": no mark
		}
		before, err := a.todoViews.Get(ctx, todo.ID().String())
		require.NoError(t, err)

		require.NoError(t, newRelay(projector).PollOnce(ctx))

		require.Empty(t, pending(t, todo.ID()))
		after, err := a.todoViews.Get(ctx, todo.ID().String())
		require.NoError(t, err)
		require.Equal(t, before, after)
		require.Equal(t, "crashed v2", after.Title)
		require.Equal(t, []string{"created", "renamed"}, actions(t, todo.ID()), "one activity per event, not two")
	})
}

// recorder is a Handler that notes each message id, then passes the message on, or fails
// after the projector has written, so a test can see that the relay's transaction rolls back.
type recorder struct {
	next outbox.Handler
	err  error
	ids  []string
}

func (r *recorder) Handle(ctx context.Context, m outbox.Message) error {
	r.ids = append(r.ids, m.ID)
	if err := r.next.Handle(ctx, m); err != nil {
		return err
	}
	return r.err
}

// now is truncated to milliseconds like the application's nowUTC, so values round-trip exactly.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func title(t *testing.T, raw string) domain.Title {
	t.Helper()
	title, err := domain.NewTitle(raw)
	require.NoError(t, err)
	return title
}

func newTodo(t *testing.T, raw string) *domain.Todo {
	t.Helper()
	return domain.NewTodo(domain.NewTodoID(), title(t, raw), now())
}

func view(title string) *query.TodoView {
	at := now()
	return &query.TodoView{ID: newID(), Title: title, CreatedAt: at, UpdatedAt: at}
}
