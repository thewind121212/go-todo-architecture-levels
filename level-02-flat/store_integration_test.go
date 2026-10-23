//go:build integration

package main

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

// failingTx writes todo and activity inside the store's own transaction
// helper and then returns errBoom, so the rollback test can check that
// nothing was persisted.
type failingTx func(ctx context.Context, todo Todo, activity Activity) error

func TestPostgresStore(t *testing.T) {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:17-alpine", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	s, err := newPostgresStore(dsn)
	if err != nil {
		t.Fatal(err)
	}

	runStoreContract(t, s, func(ctx context.Context, todo Todo, activity Activity) error {
		return s.inTx(ctx, func(tx *gorm.DB) error {
			if err := tx.Create(&todo).Error; err != nil {
				return err
			}
			if err := tx.Create(&activity).Error; err != nil {
				return err
			}
			return errBoom
		})
	})
}

func TestMongoStore(t *testing.T) {
	ctx := context.Background()
	// Transactions need a replica set, even a single-node one.
	ctr, err := mongodb.Run(ctx, "mongo:8", mongodb.WithReplicaSet("rs0"))
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The replica set advertises the container's internal IP; connect directly
	// to the mapped port instead of following it.
	s, err := newMongoStore(ctx, uri+"&directConnection=true", "todo_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.client.Disconnect(context.Background()) })

	runStoreContract(t, s, func(ctx context.Context, todo Todo, activity Activity) error {
		return s.inTx(ctx, func(ctx context.Context) error {
			if _, err := s.todos.InsertOne(ctx, todo); err != nil {
				return err
			}
			if _, err := s.activities.InsertOne(ctx, activity); err != nil {
				return err
			}
			return errBoom
		})
	})
}

// runStoreContract is the one suite both stores must pass, so they are
// guaranteed to behave the same. Subtests share one database and run in order.
func runStoreContract(t *testing.T, s store, writeThenFail failingTx) {
	ctx := context.Background()

	create := func(t *testing.T, title string) Todo {
		t.Helper()
		todo, err := s.Create(ctx, title)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		return todo
	}
	wantActions := func(t *testing.T, todoID string, want ...string) {
		t.Helper()
		activities, err := s.Activities(ctx, todoID)
		if err != nil {
			t.Fatalf("activities: %v", err)
		}
		got := []string{}
		for _, a := range activities {
			got = append(got, a.Action)
		}
		if !slices.Equal(got, want) {
			t.Errorf("actions = %v, want %v", got, want)
		}
	}

	t.Run("ping", func(t *testing.T) {
		if err := s.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("create then get round-trips exactly", func(t *testing.T) {
		todo := create(t, "  buy milk ")
		if todo.Title != "buy milk" || todo.Completed {
			t.Errorf("unexpected todo %+v", todo)
		}
		got, err := s.Get(ctx, todo.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got != todo {
			t.Errorf("get = %+v, want %+v", got, todo)
		}
		wantActions(t, todo.ID, ActionCreated)
	})

	t.Run("create rejects invalid title", func(t *testing.T) {
		if _, err := s.Create(ctx, "   "); !errors.Is(err, ErrInvalidTitle) {
			t.Errorf("err = %v, want %v", err, ErrInvalidTitle)
		}
	})

	t.Run("get unknown id", func(t *testing.T) {
		if _, err := s.Get(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("list is newest first and paged", func(t *testing.T) {
		first, second, third := create(t, "1"), create(t, "2"), create(t, "3")

		page, err := s.List(ctx, 2, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) != 2 || page[0] != third || page[1] != second {
			t.Errorf("page 1 = %+v", page)
		}
		page, err = s.List(ctx, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) != 1 || page[0] != first {
			t.Errorf("page 2 = %+v", page)
		}
		page, err = s.List(ctx, 10, 1_000_000)
		if err != nil {
			t.Fatal(err)
		}
		if page == nil || len(page) != 0 {
			t.Errorf("past the end = %#v, want empty non-nil slice", page)
		}
	})

	t.Run("update title and completed writes two activities", func(t *testing.T) {
		todo := create(t, "old")
		got, err := s.Update(ctx, todo.ID, todoPatch{Title: new(" new "), Completed: new(true)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "new" || !got.Completed || got.CreatedAt != todo.CreatedAt || got.UpdatedAt.Before(todo.UpdatedAt) {
			t.Errorf("unexpected todo %+v", got)
		}
		stored, err := s.Get(ctx, todo.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored != got {
			t.Errorf("stored = %+v, want %+v", stored, got)
		}
		wantActions(t, todo.ID, ActionCreated, ActionRenamed, ActionCompleted)

		if _, err := s.Update(ctx, todo.ID, todoPatch{Completed: new(false)}); err != nil {
			t.Fatal(err)
		}
		wantActions(t, todo.ID, ActionCreated, ActionRenamed, ActionCompleted, ActionReopened)
	})

	t.Run("update that changes nothing writes nothing", func(t *testing.T) {
		todo := create(t, "same")
		got, err := s.Update(ctx, todo.ID, todoPatch{Title: new("same"), Completed: new(false)})
		if err != nil {
			t.Fatal(err)
		}
		if got != todo {
			t.Errorf("got %+v, want unchanged %+v", got, todo)
		}
		wantActions(t, todo.ID, ActionCreated)
	})

	t.Run("update errors", func(t *testing.T) {
		todo := create(t, "x")
		if _, err := s.Update(ctx, uuid.NewString(), todoPatch{Completed: new(true)}); !errors.Is(err, ErrNotFound) {
			t.Errorf("unknown id: err = %v, want %v", err, ErrNotFound)
		}
		if _, err := s.Update(ctx, todo.ID, todoPatch{}); !errors.Is(err, ErrEmptyPatch) {
			t.Errorf("empty patch: err = %v, want %v", err, ErrEmptyPatch)
		}
		if _, err := s.Update(ctx, todo.ID, todoPatch{Title: new("")}); !errors.Is(err, ErrInvalidTitle) {
			t.Errorf("blank title: err = %v, want %v", err, ErrInvalidTitle)
		}
		wantActions(t, todo.ID, ActionCreated)
	})

	t.Run("delete keeps the activities", func(t *testing.T) {
		todo := create(t, "bye")
		if err := s.Delete(ctx, todo.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("get after delete: err = %v, want %v", err, ErrNotFound)
		}
		if err := s.Delete(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("second delete: err = %v, want %v", err, ErrNotFound)
		}
		wantActions(t, todo.ID, ActionCreated, ActionDeleted)
	})

	t.Run("activities of unknown todo is empty, not nil", func(t *testing.T) {
		activities, err := s.Activities(ctx, uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		if activities == nil || len(activities) != 0 {
			t.Errorf("activities = %#v, want empty non-nil slice", activities)
		}
	})

	t.Run("rollback persists nothing", func(t *testing.T) {
		todo, err := newTodo("never saved")
		if err != nil {
			t.Fatal(err)
		}
		activity := newActivity(todo.ID, ActionCreated, todo.CreatedAt)

		if err := writeThenFail(ctx, todo, activity); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want %v", err, errBoom)
		}
		if _, err := s.Get(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("todo was persisted: err = %v", err)
		}
		wantActions(t, todo.ID)
	})
}
