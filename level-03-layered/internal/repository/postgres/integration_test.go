//go:build integration

package postgres_test

import (
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/level03/internal/repository/postgres"
	"example.com/todo/level03/internal/repository/repotest"
)

func TestContract(t *testing.T) {
	ctr, err := tcpostgres.Run(t.Context(), "postgres:17-alpine",
		tcpostgres.WithDatabase("todo"),
		tcpostgres.WithUsername("todo"),
		tcpostgres.WithPassword("todo"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := ctr.ConnectionString(t.Context(), "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(db); err != nil {
		t.Fatal(err)
	}

	repotest.Run(t, func(t *testing.T) repotest.Storage {
		if err := db.Exec("TRUNCATE todos, activities").Error; err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return repotest.Storage{
			Tx:         postgres.NewTxManager(db),
			Todos:      postgres.NewTodoRepository(db),
			Activities: postgres.NewActivityRepository(db),
		}
	})
}
