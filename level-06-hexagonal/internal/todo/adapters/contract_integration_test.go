//go:build integration

// Package adapters_test runs ONE contract suite against BOTH driven adapters,
// so you can see that Postgres and Mongo honor the ports identically.
package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/level06/internal/infrastructure/database"
	mongoadapter "example.com/todo/level06/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level06/internal/todo/adapters/postgres"
	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// adapters is one storage choice, seen only through the driven ports.
type adapters struct {
	todos      ports.TodoRepository
	activities ports.ActivityRepository
	tx         ports.TxManager
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
		activities: pgadapter.NewActivityRepository(db),
		tx:         pgadapter.NewTxManager(db),
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
		activities: mongoadapter.NewActivityRepository(db),
		tx:         mongoadapter.NewTxManager(db),
	})
}

// runContract is the behavior every adapter must have. Subtests share one
// database and run in order.
func runContract(t *testing.T, a adapters) {
	ctx := context.Background()

	t.Run("create then get round-trips every field", func(t *testing.T) {
		todo := newTodo(t, "round trip")
		require.NoError(t, a.todos.Create(ctx, todo))

		got, err := a.todos.Get(ctx, todo.ID)
		require.NoError(t, err)
		require.Equal(t, todo, got)
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		missing := newTodo(t, "missing")
		_, err := a.todos.Get(ctx, missing.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.ErrorIs(t, a.todos.Update(ctx, missing), domain.ErrNotFound)
		require.ErrorIs(t, a.todos.Delete(ctx, missing.ID), domain.ErrNotFound)
	})

	t.Run("list is newest first and paginates", func(t *testing.T) {
		first, second, third := newTodo(t, "1"), newTodo(t, "2"), newTodo(t, "3")
		for _, todo := range []*domain.Todo{first, second, third} {
			require.NoError(t, a.todos.Create(ctx, todo))
		}

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

	t.Run("update persists title and completed", func(t *testing.T) {
		todo := newTodo(t, "before")
		require.NoError(t, a.todos.Create(ctx, todo))
		_, err := todo.Rename("after")
		require.NoError(t, err)
		todo.SetCompleted(true)
		require.NoError(t, a.todos.Update(ctx, todo))

		got, err := a.todos.Get(ctx, todo.ID)
		require.NoError(t, err)
		require.Equal(t, todo, got)

		todo.SetCompleted(false)
		require.NoError(t, a.todos.Update(ctx, todo))
		got, err = a.todos.Get(ctx, todo.ID)
		require.NoError(t, err)
		require.False(t, got.Completed)
	})

	t.Run("delete removes the todo", func(t *testing.T) {
		todo := newTodo(t, "doomed")
		require.NoError(t, a.todos.Create(ctx, todo))
		require.NoError(t, a.todos.Delete(ctx, todo.ID))

		_, err := a.todos.Get(ctx, todo.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("activities list oldest first", func(t *testing.T) {
		todo := newTodo(t, "audited")
		created := newActivity(t, todo.ID, domain.ActionCreated)
		renamed := newActivity(t, todo.ID, domain.ActionRenamed)
		other := newActivity(t, newTodo(t, "other").ID, domain.ActionCreated)
		for _, act := range []*domain.Activity{created, renamed, other} {
			require.NoError(t, a.activities.Append(ctx, act))
		}

		got, err := a.activities.ListByTodo(ctx, todo.ID)
		require.NoError(t, err)
		require.Equal(t, []*domain.Activity{created, renamed}, got)

		got, err = a.activities.ListByTodo(ctx, newTodo(t, "none").ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})

	t.Run("WithinTx commits todo and activity together", func(t *testing.T) {
		todo := newTodo(t, "committed")
		err := a.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := a.todos.Create(ctx, todo); err != nil {
				return err
			}
			return a.activities.Append(ctx, newActivity(t, todo.ID, domain.ActionCreated))
		})
		require.NoError(t, err)

		_, err = a.todos.Get(ctx, todo.ID)
		require.NoError(t, err)
		got, err := a.activities.ListByTodo(ctx, todo.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})

	t.Run("WithinTx rolls back everything on error", func(t *testing.T) {
		errBoom := errors.New("boom")
		todo := newTodo(t, "rolled back")
		err := a.tx.WithinTx(ctx, func(ctx context.Context) error {
			require.NoError(t, a.todos.Create(ctx, todo))
			require.NoError(t, a.activities.Append(ctx, newActivity(t, todo.ID, domain.ActionCreated)))
			// Inside the transaction the write is visible...
			_, err := a.todos.Get(ctx, todo.ID)
			require.NoError(t, err)
			return errBoom
		})
		require.ErrorIs(t, err, errBoom)

		// ...outside it, nothing was persisted.
		_, err = a.todos.Get(ctx, todo.ID)
		require.ErrorIs(t, err, domain.ErrNotFound)
		got, err := a.activities.ListByTodo(ctx, todo.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func newTodo(t *testing.T, title string) *domain.Todo {
	t.Helper()
	todo, err := domain.NewTodo(title)
	require.NoError(t, err)
	return todo
}

func newActivity(t *testing.T, todoID string, action domain.Action) *domain.Activity {
	t.Helper()
	activity, err := domain.NewActivity(todoID, action)
	require.NoError(t, err)
	return activity
}
