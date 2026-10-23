//go:build integration

package adapter_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/level05/internal/adapter/mongo"
	"example.com/todo/level05/internal/adapter/postgres"
)

// The containers are started by the tests themselves; the compose databases are not used.

func TestPostgresAdapter(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("todo"),
		tcpostgres.WithUsername("todo"),
		tcpostgres.WithPassword("todo"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	runContract(t, store{
		todos:      postgres.NewTodoRepository(db),
		activities: postgres.NewActivityRepository(db),
		tx:         postgres.NewTxManager(db),
	})
}

func TestMongoAdapter(t *testing.T) {
	ctx := context.Background()
	// Mongo supports transactions only on a replica set.
	ctr, err := tcmongo.Run(ctx, "mongo:8", tcmongo.WithReplicaSet("rs0"))
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// directConnection: the replica set advertises the container's internal address, so talk to it directly.
	db, err := mongo.Open(ctx, uri+"&directConnection=true", "todo_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Client().Disconnect(context.Background()) })

	runContract(t, store{
		todos:      mongo.NewTodoRepository(db),
		activities: mongo.NewActivityRepository(db),
		tx:         mongo.NewTxManager(db),
	})
}
