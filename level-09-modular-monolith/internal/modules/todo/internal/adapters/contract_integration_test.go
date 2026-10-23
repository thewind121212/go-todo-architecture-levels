//go:build integration

// Package adapters_test runs ONE contract suite against BOTH of the todo module's driven
// adapters, so you can see that Postgres and Mongo honor the repository and the ports identically.
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

	todomodule "example.com/todo/level09/internal/modules/todo"
	mongoadapter "example.com/todo/level09/internal/modules/todo/internal/adapters/mongo"
	pgadapter "example.com/todo/level09/internal/modules/todo/internal/adapters/postgres"
	"example.com/todo/level09/internal/modules/todo/internal/application/integration"
	"example.com/todo/level09/internal/modules/todo/internal/application/projection"
	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/modules/todo/internal/codec"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/database"
	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/outbox"
)

// adapters is one storage choice, seen only through the repository and the ports.
type adapters struct {
	todos     domain.TodoRepository
	outbox    outbox.Store
	tx        projection.TxManager
	todoViews query.TodoViews
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

	// OpenPostgres applies the module's real embedded SQL migrations, like the app does.
	db, closeDB, err := database.OpenPostgres(ctx, dsn, todomodule.PostgresMigrations())
	require.NoError(t, err)
	t.Cleanup(closeDB)

	runContract(t, adapters{
		todos:     pgadapter.NewTodoRepository(db),
		outbox:    pgadapter.NewOutbox(db),
		tx:        pgadapter.NewTxManager(db),
		todoViews: pgadapter.NewTodoViews(db),
		rejectOutbox: func(t *testing.T, id domain.TodoID) {
			// DDL takes no bind parameters; id is a parsed UUID, so formatting it in is safe.
			check := fmt.Sprintf("ALTER TABLE todo.outbox ADD CONSTRAINT reject_todo CHECK (todo_id <> '%s')", id)
			require.NoError(t, db.Exec(check).Error)
			t.Cleanup(func() { require.NoError(t, db.Exec("ALTER TABLE todo.outbox DROP CONSTRAINT reject_todo").Error) })
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
	db, disconnect, err := database.OpenMongo(ctx, uri+"&directConnection=true", "todo_test", todomodule.PrepareMongo)
	require.NoError(t, err)
	t.Cleanup(disconnect)

	runContract(t, adapters{
		todos:     mongoadapter.NewTodoRepository(db),
		outbox:    mongoadapter.NewOutbox(db),
		tx:        mongoadapter.NewTxManager(db),
		todoViews: mongoadapter.NewTodoViews(db),
		rejectOutbox: func(t *testing.T, id domain.TodoID) {
			validator := func(rule bson.D) bson.D {
				return bson.D{{Key: "collMod", Value: "todo.outbox"}, {Key: "validator", Value: rule}}
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
			if m.AggregateID == id.String() {
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
	// saveNew stores a new todo and returns it as stored (a Todo serves one command).
	saveNew := func(t *testing.T, title string) *domain.Todo {
		t.Helper()
		todo := newTodo(t, title)
		require.NoError(t, a.todos.Save(ctx, todo))
		return load(t, todo.ID())
	}
	// publisher is the module's relay handler over a bus on which the real projector
	// listens, as in the app; newRelay is the relay over this storage.
	bus := eventbus.New()
	projection.NewProjector(a.todoViews, a.tx).Subscribe(bus)
	publisher := integration.NewPublisher(bus)
	newRelay := func(handler outbox.Handler) *outbox.Relay {
		return outbox.NewRelay(a.outbox, handler, time.Second, zerolog.Nop())
	}
	getView := func(t *testing.T, id domain.TodoID) *query.TodoView {
		t.Helper()
		view, err := a.todoViews.Get(ctx, id.String())
		require.NoError(t, err)
		return view
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
		event, err := codec.Decode(messages[0])
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

	// ---- The relay: outbox -> projector -> read models ----

	t.Run("mark processed removes a message from the unprocessed list", func(t *testing.T) {
		id := saveNew(t, "marked").ID()
		m := pending(t, id)[0]

		require.NoError(t, a.outbox.MarkProcessed(ctx, m.ID, now()))

		require.Empty(t, pending(t, id))
	})

	t.Run("relay publishes in id order, the projector applies it, and processed events are skipped", func(t *testing.T) {
		todo := saveNew(t, "relayed")
		todo.Rename(title(t, "relayed v2"), now())
		todo.Complete(now())
		require.NoError(t, a.todos.Save(ctx, todo))
		rec := &recorder{next: publisher}

		require.NoError(t, newRelay(rec).PollOnce(ctx))

		require.NotEmpty(t, rec.ids)
		require.True(t, slices.IsSorted(rec.ids), "oldest event first")
		require.Empty(t, pending(t, todo.ID()))
		require.Equal(t, &query.TodoView{
			ID:        todo.ID().String(),
			Title:     "relayed v2",
			Completed: true,
			CreatedAt: todo.CreatedAt(),
			UpdatedAt: todo.UpdatedAt(),
		}, getView(t, todo.ID()))

		again := &recorder{next: publisher}
		require.NoError(t, newRelay(again).PollOnce(ctx))
		require.Empty(t, again.ids, "processed events are not handed over again")
	})

	// The projector's transaction: a failure after its writes rolls them back.
	t.Run("a failed projection rolls its view writes back and leaves the event unprocessed", func(t *testing.T) {
		id := saveNew(t, "failing").ID()
		failing := eventbus.New()
		projection.NewProjector(a.todoViews, failAfter{a.tx}).Subscribe(failing)

		require.ErrorIs(t, newRelay(integration.NewPublisher(failing)).PollOnce(ctx), errBoom)

		require.Equal(t, []string{"created"}, pendingNames(t, id))
		_, err := a.todoViews.Get(ctx, id.String())
		require.ErrorIs(t, err, query.ErrNotFound, "the projector's upsert was rolled back")

		require.NoError(t, newRelay(publisher).PollOnce(ctx), "retried on the next poll")
		require.Empty(t, pending(t, id))
		require.Equal(t, "failing", getView(t, id).Title)
	})

	// The level's core lesson, on a real database: subscribers commit one by one. When a
	// later subscriber fails, the projector's commit stays, the event stays unprocessed,
	// and the redelivery that follows changes nothing for the projector.
	t.Run("a failing subscriber does not undo another's commit, and the redelivery is harmless", func(t *testing.T) {
		id := saveNew(t, "half delivered").ID()
		withFailing := eventbus.New()
		projection.NewProjector(a.todoViews, a.tx).Subscribe(withFailing)
		withFailing.Subscribe("todo.created", func(context.Context, eventbus.Event) error { return errBoom })

		require.ErrorIs(t, newRelay(integration.NewPublisher(withFailing)).PollOnce(ctx), errBoom)
		require.Equal(t, []string{"created"}, pendingNames(t, id), "not marked: another subscriber failed")
		before := getView(t, id) // the projector committed on its own

		require.NoError(t, newRelay(publisher).PollOnce(ctx))

		require.Empty(t, pending(t, id))
		require.Equal(t, before, getView(t, id))
	})

	// At-least-once delivery: an event can be delivered and then fail to be marked, here by
	// publishing it outside the relay. Delivering it again must change nothing.
	t.Run("an event delivered but not marked is delivered again harmlessly", func(t *testing.T) {
		todo := saveNew(t, "crashed")
		todo.Rename(title(t, "crashed v2"), now())
		require.NoError(t, a.todos.Save(ctx, todo))
		for _, m := range pending(t, todo.ID()) {
			require.NoError(t, publisher.Handle(ctx, m)) // delivered, then "crash": no mark
		}
		before := getView(t, todo.ID())

		require.NoError(t, newRelay(publisher).PollOnce(ctx))

		require.Empty(t, pending(t, todo.ID()))
		after := getView(t, todo.ID())
		require.Equal(t, before, after)
		require.Equal(t, "crashed v2", after.Title)
	})
}

var errBoom = errors.New("boom")

// failAfter runs fn in the real transaction, then fails, so the transaction rolls back
// after the projector has written.
type failAfter struct{ tx projection.TxManager }

func (f failAfter) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return f.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}
		return errBoom
	})
}

// recorder is a Handler that notes each message id, then passes the message on.
type recorder struct {
	next outbox.Handler
	ids  []string
}

func (r *recorder) Handle(ctx context.Context, m outbox.Message) error {
	r.ids = append(r.ids, m.ID)
	return r.next.Handle(ctx, m)
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
