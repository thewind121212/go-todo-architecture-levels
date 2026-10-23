//go:build integration

// Package adapters_test runs ONE contract suite against BOTH of the activity module's
// driven adapters, so you can see that Postgres and Mongo behave identically.
package adapters_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/level09/internal/modules/activity"
	mongoadapter "example.com/todo/level09/internal/modules/activity/internal/adapters/mongo"
	pgadapter "example.com/todo/level09/internal/modules/activity/internal/adapters/postgres"
	"example.com/todo/level09/internal/modules/activity/internal/application"
	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/platform/database"
)

// adapters is one storage choice, seen only through the module's ports.
type adapters struct {
	activities application.ActivityLog
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

	// Only this module's migrations: it needs no table of the todo module.
	db, closeDB, err := database.OpenPostgres(ctx, dsn, activity.PostgresMigrations())
	require.NoError(t, err)
	t.Cleanup(closeDB)

	runContract(t, adapters{activities: pgadapter.NewActivityLog(db)})
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
	db, disconnect, err := database.OpenMongo(ctx, uri+"&directConnection=true", "todo_test", activity.PrepareMongo)
	require.NoError(t, err)
	t.Cleanup(disconnect)

	runContract(t, adapters{activities: mongoadapter.NewActivityLog(db)})
}

// runContract is the behavior every adapter must have. Subtests share one
// database and run in order.
func runContract(t *testing.T, a adapters) {
	ctx := context.Background()
	list := func(t *testing.T, todoID string) []*application.Activity {
		t.Helper()
		got, err := a.activities.ListByTodo(ctx, todoID)
		require.NoError(t, err)
		return got
	}

	t.Run("add ignores a repeated id and lists oldest first, per todo", func(t *testing.T) {
		todoID := uuid.NewString()
		first := &application.Activity{ID: newID(), TodoID: todoID, Action: "created", CreatedAt: now()}
		second := &application.Activity{ID: newID(), TodoID: todoID, Action: "renamed", CreatedAt: now()}
		other := &application.Activity{ID: newID(), TodoID: uuid.NewString(), Action: "created", CreatedAt: now()}
		for _, act := range []*application.Activity{first, second, other} {
			require.NoError(t, a.activities.Add(ctx, act))
		}
		repeat := *first
		repeat.Action = "renamed"
		require.NoError(t, a.activities.Add(ctx, &repeat), "a repeated id is not an error")

		require.Equal(t, []*application.Activity{first, second}, list(t, todoID), "the first write wins")

		none := list(t, uuid.NewString())
		require.NotNil(t, none)
		require.Empty(t, none)
	})

	// Idempotency, end to end through the subscriber: at-least-once delivery may hand
	// the module the same event twice, and the log still has one line for it.
	t.Run("recording the same event twice leaves one activity", func(t *testing.T) {
		svc := application.NewService(a.activities)
		event := contracts.TodoRenamed{
			EventMeta: contracts.EventMeta{EventID: newID(), TodoID: uuid.NewString(), OccurredAt: now()},
			Title:     "new",
		}

		require.NoError(t, svc.Record(ctx, event))
		require.NoError(t, svc.Record(ctx, event))

		require.Equal(t, []*application.Activity{{
			ID:        event.EventID,
			TodoID:    event.TodoID,
			Action:    "renamed",
			CreatedAt: event.OccurredAt,
		}}, list(t, event.TodoID))
	})
}

// now is truncated to milliseconds like the todo module's nowUTC, so values round-trip exactly.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}
