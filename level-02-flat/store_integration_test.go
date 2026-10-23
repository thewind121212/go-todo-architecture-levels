//go:build integration

// store_integration_test.go: one contract suite run against both stores in testcontainers.

package main

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

func TestPostgresStore(t *testing.T) {
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
	s, err := newPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}

	runStoreContract(t, s, func(ctx context.Context, todo Todo) error {
		return s.inTx(ctx, func(tx *gorm.DB) error {
			if err := tx.Create(&todo).Error; err != nil {
				return err
			}
			a := newActivity(todo.ID, ActionCreated, todo.CreatedAt)
			if err := tx.Create(&a).Error; err != nil {
				return err
			}
			return errBoom
		})
	})
}

func TestMongoStore(t *testing.T) {
	ctx := context.Background()
	// Mongo only supports transactions on a replica set, even a single-node one.
	ctr, err := tcmongo.Run(ctx, "mongo:8", tcmongo.WithReplicaSet("rs0"))
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newMongoStore(ctx, uri, "todo_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.client.Disconnect(context.Background()) })

	runStoreContract(t, s, func(ctx context.Context, todo Todo) error {
		return s.inTx(ctx, func(ctx context.Context) error {
			if _, err := s.todos.InsertOne(ctx, todo); err != nil {
				return err
			}
			if _, err := s.activities.InsertOne(ctx, newActivity(todo.ID, ActionCreated, todo.CreatedAt)); err != nil {
				return err
			}
			return errBoom
		})
	})
}

// runStoreContract is the one test suite both stores must pass, so they are
// guaranteed to behave the same. createThenFail must insert a todo and its
// "created" activity inside one transaction, then return errBoom before commit.
func runStoreContract(t *testing.T, s store, createThenFail func(ctx context.Context, t Todo) error) {
	ctx := context.Background()

	mustCreate := func(t *testing.T, title string) Todo {
		t.Helper()
		todo, err := newTodo(title)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Create(ctx, todo); err != nil {
			t.Fatalf("create: %v", err)
		}
		return todo
	}
	actionsOf := func(t *testing.T, id string) []Action {
		t.Helper()
		activities, err := s.ListActivities(ctx, id)
		if err != nil {
			t.Fatalf("list activities: %v", err)
		}
		actions := []Action{}
		for _, a := range activities {
			actions = append(actions, a.Action)
		}
		return actions
	}

	t.Run("ping", func(t *testing.T) {
		if err := s.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("create then get round-trips exactly", func(t *testing.T) {
		want := mustCreate(t, "round trip")
		got, err := s.Get(ctx, want.ID)
		if err != nil {
			t.Fatal(err)
		}
		// == also compares time zones, so this catches non-UTC timestamps.
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		if actions := actionsOf(t, want.ID); !slices.Equal(actions, []Action{ActionCreated}) {
			t.Fatalf("actions = %v", actions)
		}
	})

	t.Run("get unknown is not found", func(t *testing.T) {
		if _, err := s.Get(ctx, newID()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("list is newest first and paged", func(t *testing.T) {
		a, b, c := mustCreate(t, "a"), mustCreate(t, "b"), mustCreate(t, "c")
		page, err := s.List(ctx, 3, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(page, []Todo{c, b, a}) {
			t.Fatalf("page = %+v, want c, b, a", page)
		}
		page, err = s.List(ctx, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(page, []Todo{b}) {
			t.Fatalf("page = %+v, want b", page)
		}
		page, err = s.List(ctx, 10, 1_000_000)
		if err != nil {
			t.Fatal(err)
		}
		if page == nil || len(page) != 0 {
			t.Fatalf("page past the end = %#v, want empty non-nil", page)
		}
	})

	t.Run("update writes one activity per change", func(t *testing.T) {
		todo := mustCreate(t, "old")
		got, err := s.Update(ctx, todo.ID, Patch{Title: new("new"), Completed: new(true)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "new" || !got.Completed || got.UpdatedAt.Before(todo.UpdatedAt) {
			t.Fatalf("updated = %+v", got)
		}
		stored, err := s.Get(ctx, todo.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored != got {
			t.Fatalf("stored %+v, returned %+v", stored, got)
		}

		unchanged, err := s.Update(ctx, todo.ID, Patch{Title: new("new"), Completed: new(true)})
		if err != nil {
			t.Fatal(err)
		}
		if unchanged != got {
			t.Fatalf("no-op update changed the todo: %+v", unchanged)
		}

		if _, err := s.Update(ctx, todo.ID, Patch{Completed: new(false)}); err != nil {
			t.Fatal(err)
		}
		want := []Action{ActionCreated, ActionRenamed, ActionCompleted, ActionReopened}
		if actions := actionsOf(t, todo.ID); !slices.Equal(actions, want) {
			t.Fatalf("actions = %v, want %v", actions, want)
		}
	})

	t.Run("update unknown is not found", func(t *testing.T) {
		if _, err := s.Update(ctx, newID(), Patch{Completed: new(true)}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete keeps the activities", func(t *testing.T) {
		todo := mustCreate(t, "doomed")
		if err := s.Delete(ctx, todo.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("get after delete: err = %v, want ErrNotFound", err)
		}
		if err := s.Delete(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("second delete: err = %v, want ErrNotFound", err)
		}
		want := []Action{ActionCreated, ActionDeleted}
		if actions := actionsOf(t, todo.ID); !slices.Equal(actions, want) {
			t.Fatalf("actions = %v, want %v", actions, want)
		}
	})

	t.Run("activities of unknown todo are empty", func(t *testing.T) {
		activities, err := s.ListActivities(ctx, newID())
		if err != nil {
			t.Fatal(err)
		}
		if activities == nil || len(activities) != 0 {
			t.Fatalf("activities = %#v, want empty non-nil", activities)
		}
	})

	t.Run("failed transaction persists nothing", func(t *testing.T) {
		todo, err := newTodo("never saved")
		if err != nil {
			t.Fatal(err)
		}
		if err := createThenFail(ctx, todo); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want errBoom", err)
		}
		if _, err := s.Get(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("todo was persisted: err = %v", err)
		}
		if actions := actionsOf(t, todo.ID); len(actions) != 0 {
			t.Fatalf("activities were persisted: %v", actions)
		}
	})
}
