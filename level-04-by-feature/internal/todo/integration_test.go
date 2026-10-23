//go:build integration

package todo

import (
	"context"
	"errors"
	"testing"

	"example.com/todo/level04/internal/activity"
	"example.com/todo/level04/internal/platform/testdb"
	"example.com/todo/level04/internal/platform/tx"
)

// backend is one database's todo repository, activity repository and
// transaction manager. Both databases run the same contract below.
type backend struct {
	todos      Repository
	activities activity.Repository
	tx         TxManager
}

func TestPostgres(t *testing.T) {
	db := testdb.Postgres(t)
	ctx := context.Background()
	if err := MigratePostgres(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := activity.MigratePostgres(ctx, db); err != nil {
		t.Fatal(err)
	}
	runContract(t, backend{
		todos:      NewPostgresRepository(db),
		activities: activity.NewPostgresRepository(db),
		tx:         tx.NewPostgresManager(db),
	})
}

func TestMongo(t *testing.T) {
	client := testdb.Mongo(t)
	db := client.Database("todo")
	ctx := context.Background()
	if err := MigrateMongo(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := activity.MigrateMongo(ctx, db); err != nil {
		t.Fatal(err)
	}
	runContract(t, backend{
		todos:      NewMongoRepository(db),
		activities: activity.NewMongoRepository(db),
		tx:         tx.NewMongoManager(client),
	})
}

func runContract(t *testing.T, b backend) {
	ctx := context.Background()
	mustCreate := func(t *testing.T, title string) Todo {
		t.Helper()
		td, err := New(title)
		if err != nil {
			t.Fatal(err)
		}
		if err := b.todos.Create(ctx, td); err != nil {
			t.Fatalf("Create: %v", err)
		}
		return td
	}
	unknownID := "0190a0e4-0000-7000-8000-000000000000"

	t.Run("create then get round-trips every field", func(t *testing.T) {
		want := mustCreate(t, "round trip")
		got, err := b.todos.Get(ctx, want.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("get unknown id", func(t *testing.T) {
		if _, err := b.todos.Get(ctx, unknownID); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("list is newest first and pages", func(t *testing.T) {
		older := mustCreate(t, "older")
		newer := mustCreate(t, "newer")
		page, err := b.todos.List(ctx, 2, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page) != 2 || page[0].ID != newer.ID || page[1].ID != older.ID {
			t.Errorf("List(2, 0) = %+v, want [newer older]", page)
		}
		page, err = b.todos.List(ctx, 1, 1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page) != 1 || page[0].ID != older.ID {
			t.Errorf("List(1, 1) = %+v, want [older]", page)
		}
		page, err = b.todos.List(ctx, 10, 1000)
		if err != nil || page == nil || len(page) != 0 {
			t.Errorf("List past the end = %v, %v; want empty, non-nil", page, err)
		}
	})

	t.Run("update", func(t *testing.T) {
		td := mustCreate(t, "before")
		if _, err := td.Apply(Patch{Title: new("after"), Completed: new(true)}); err != nil {
			t.Fatal(err)
		}
		if err := b.todos.Update(ctx, td); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := b.todos.Get(ctx, td.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != td {
			t.Errorf("got %+v, want %+v", got, td)
		}
	})

	t.Run("update unknown id", func(t *testing.T) {
		if err := b.todos.Update(ctx, Todo{ID: unknownID, Title: "x"}); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("delete", func(t *testing.T) {
		td := mustCreate(t, "doomed")
		if err := b.todos.Delete(ctx, td.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := b.todos.Get(ctx, td.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get after delete error = %v, want %v", err, ErrNotFound)
		}
		if err := b.todos.Delete(ctx, td.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("second Delete error = %v, want %v", err, ErrNotFound)
		}
	})

	// The next two tests are the level's core lesson: one transaction spans the
	// todo feature and the activity feature.
	recorder := activity.NewService(b.activities)
	writeBoth := func(ctx context.Context, td Todo) error {
		if err := b.todos.Create(ctx, td); err != nil {
			return err
		}
		return recorder.Record(ctx, td.ID, ActionCreated)
	}

	t.Run("commit persists todo and activity", func(t *testing.T) {
		td, _ := New("committed")
		if err := b.tx.WithinTx(ctx, func(ctx context.Context) error { return writeBoth(ctx, td) }); err != nil {
			t.Fatalf("WithinTx: %v", err)
		}
		if _, err := b.todos.Get(ctx, td.ID); err != nil {
			t.Errorf("todo not committed: %v", err)
		}
		if acts, err := b.activities.ListByTodo(ctx, td.ID); err != nil || len(acts) != 1 {
			t.Errorf("activities = %v, %v; want 1", acts, err)
		}
	})

	t.Run("rollback discards todo and activity", func(t *testing.T) {
		td, _ := New("rolled back")
		errBoom := errors.New("boom")
		err := b.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := writeBoth(ctx, td); err != nil {
				return err
			}
			return errBoom
		})
		if !errors.Is(err, errBoom) {
			t.Fatalf("WithinTx error = %v, want %v", err, errBoom)
		}
		if _, err := b.todos.Get(ctx, td.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("todo persisted after rollback: %v", err)
		}
		if acts, err := b.activities.ListByTodo(ctx, td.ID); err != nil || len(acts) != 0 {
			t.Errorf("activities after rollback = %v, %v; want none", acts, err)
		}
	})
}
