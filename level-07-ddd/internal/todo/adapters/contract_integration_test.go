//go:build integration

// Package adapters_test runs ONE contract suite against BOTH driven adapters,
// so you can see that Postgres and Mongo honor the repository and the port identically.
package adapters_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/v2/bson"

	"example.com/todo/level07/internal/infrastructure/database"
	mongoadapter "example.com/todo/level07/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level07/internal/todo/adapters/postgres"
	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
)

// adapters is one storage choice, seen only through the repository and the port.
type adapters struct {
	todos      domain.TodoRepository
	activities ports.ActivityLog
	// rejectActivities makes the database refuse activities for one todo until the test
	// ends. It is the only storage-specific hook: it lets the suite break Save halfway.
	rejectActivities func(t *testing.T, id domain.TodoID)
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
		activities: pgadapter.NewActivityLog(db),
		rejectActivities: func(t *testing.T, id domain.TodoID) {
			// DDL takes no bind parameters; id is a parsed UUID, so formatting it in is safe.
			check := fmt.Sprintf("ALTER TABLE activities ADD CONSTRAINT reject_todo CHECK (todo_id <> '%s')", id)
			require.NoError(t, db.Exec(check).Error)
			t.Cleanup(func() { require.NoError(t, db.Exec("ALTER TABLE activities DROP CONSTRAINT reject_todo").Error) })
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
		activities: mongoadapter.NewActivityLog(db),
		rejectActivities: func(t *testing.T, id domain.TodoID) {
			validator := func(rule bson.D) bson.D {
				return bson.D{{Key: "collMod", Value: "activities"}, {Key: "validator", Value: rule}}
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
	actions := func(t *testing.T, id domain.TodoID) []domain.Action {
		t.Helper()
		activities, err := a.activities.ListByTodo(ctx, id)
		require.NoError(t, err)
		var actions []domain.Action
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

	t.Run("save then get round-trips every field", func(t *testing.T) {
		todo := newTodo(t, "round trip")
		require.NoError(t, a.todos.Save(ctx, todo))

		got := load(t, todo.ID())
		require.Equal(t, 1, got.Version(), "the first save stores version 1")
		require.Equal(t, todo.ID(), got.ID())
		require.Equal(t, todo.Title(), got.Title())
		require.Equal(t, todo.Completed(), got.Completed())
		require.Equal(t, todo.CreatedAt(), got.CreatedAt())
		require.Equal(t, todo.UpdatedAt(), got.UpdatedAt())

		activities, err := a.activities.ListByTodo(ctx, todo.ID())
		require.NoError(t, err)
		require.Len(t, activities, 1)
		require.Equal(t, domain.ActionCreated, activities[0].Action)
		require.Equal(t, todo.ID().String(), activities[0].TodoID)
		require.Equal(t, todo.CreatedAt(), activities[0].CreatedAt, "the event's time")
		require.Equal(t, uuid.Version(7), uuid.MustParse(activities[0].ID).Version(), "the event's id")
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		_, err := a.todos.Get(ctx, domain.NewTodoID())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("list is newest first and paginates", func(t *testing.T) {
		first, second, third := saveNew(t, "1"), saveNew(t, "2"), saveNew(t, "3")

		page, err := a.todos.List(ctx, 2, 0)
		require.NoError(t, err)
		require.Equal(t, []*domain.Todo{third, second}, page)

		page, err = a.todos.List(ctx, 1, 2)
		require.NoError(t, err)
		require.Equal(t, []*domain.Todo{first}, page)

		page, err = a.todos.List(ctx, 10, 1000)
		require.NoError(t, err)
		require.NotNil(t, page)
		require.Empty(t, page)
	})

	t.Run("save persists each change with its events and bumps the version", func(t *testing.T) {
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

		require.Equal(t, []domain.Action{domain.ActionCreated, domain.ActionRenamed, domain.ActionCompleted, domain.ActionReopened}, actions(t, id))
	})

	t.Run("save without pending events writes nothing", func(t *testing.T) {
		id := saveNew(t, "unchanged").ID()

		todo := load(t, id)
		todo.Rename(title(t, "unchanged"), now())
		require.NoError(t, a.todos.Save(ctx, todo))

		require.Equal(t, 1, load(t, id).Version())
		require.Equal(t, []domain.Action{domain.ActionCreated}, actions(t, id))
	})

	t.Run("delete removes the todo and keeps its activities", func(t *testing.T) {
		id := saveNew(t, "doomed").ID()

		todo := load(t, id)
		todo.Delete(now())
		require.NoError(t, a.todos.Save(ctx, todo))

		_, err := a.todos.Get(ctx, id)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, []domain.Action{domain.ActionCreated, domain.ActionDeleted}, actions(t, id))
	})

	t.Run("activities list oldest first, per todo", func(t *testing.T) {
		todo := saveNew(t, "audited")
		saveNew(t, "other")
		todo.Rename(title(t, "audited v2"), now())
		require.NoError(t, a.todos.Save(ctx, todo))

		require.Equal(t, []domain.Action{domain.ActionCreated, domain.ActionRenamed}, actions(t, todo.ID()))

		none, err := a.activities.ListByTodo(ctx, domain.NewTodoID())
		require.NoError(t, err)
		require.NotNil(t, none)
		require.Empty(t, none)
	})

	// Optimistic locking: a save only succeeds on the version it loaded.
	t.Run("a stale save is ErrConflict and writes nothing", func(t *testing.T) {
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
		require.Equal(t, []domain.Action{domain.ActionCreated, domain.ActionRenamed}, actions(t, id))
	})

	// The aggregate is the transaction boundary: if its events cannot be stored,
	// its state is not stored either.
	t.Run("save rolls the state back when the event append fails", func(t *testing.T) {
		todo := newTodo(t, "rolled back")
		a.rejectActivities(t, todo.ID())

		// The todo insert succeeded; the activities insert after it failed.
		require.ErrorContains(t, a.todos.Save(ctx, todo), "insert activities")

		_, err := a.todos.Get(ctx, todo.ID())
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Empty(t, actions(t, todo.ID()))
	})
}

// now is truncated to milliseconds like the application's nowUTC, so values round-trip exactly.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
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
