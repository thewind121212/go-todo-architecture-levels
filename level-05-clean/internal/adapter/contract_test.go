//go:build integration

package adapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

// store is one adapter's implementation of the use case ports.
type store struct {
	todos      usecase.TodoRepository
	activities usecase.ActivityRepository
	tx         usecase.TxManager
}

var errBoom = errors.New("boom")

// runContract is the shared suite: every adapter must pass it unchanged, which is what lets the use
// cases ignore which database they run on. Subtests share one database, so they use fresh ids.
func runContract(t *testing.T, s store) {
	ctx := context.Background()

	t.Run("create then get returns the same todo", func(t *testing.T) {
		todo := mustCreate(t, s, "write tests")
		got, err := s.todos.Get(ctx, todo.ID)
		if err != nil || got != todo {
			t.Errorf("got %+v, %v; want %+v", got, err, todo)
		}
	})

	t.Run("unknown id is not found", func(t *testing.T) {
		id := uuid.NewString()
		if _, err := s.todos.Get(ctx, id); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("Get err = %v, want ErrNotFound", err)
		}
		if err := s.todos.Delete(ctx, id); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("Delete err = %v, want ErrNotFound", err)
		}
		if err := s.todos.Update(ctx, mustNewTodo(t, "ghost")); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("Update err = %v, want ErrNotFound", err)
		}
	})

	t.Run("get for update reads inside a transaction", func(t *testing.T) {
		todo := mustCreate(t, s, "locked")
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			got, err := s.todos.GetForUpdate(ctx, todo.ID)
			if err != nil || got != todo {
				t.Errorf("got %+v, %v; want %+v", got, err, todo)
			}
			if _, err := s.todos.GetForUpdate(ctx, uuid.NewString()); !errors.Is(err, entity.ErrNotFound) {
				t.Errorf("unknown id err = %v, want ErrNotFound", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("update persists every field", func(t *testing.T) {
		todo := mustCreate(t, s, "old")
		later := todo.UpdatedAt.Add(time.Second)
		if _, err := todo.Rename("new", later); err != nil {
			t.Fatal(err)
		}
		todo.SetCompleted(true, later)
		if err := s.todos.Update(ctx, todo); err != nil {
			t.Fatal(err)
		}
		got, err := s.todos.Get(ctx, todo.ID)
		if err != nil || got != todo {
			t.Errorf("got %+v, %v; want %+v", got, err, todo)
		}
	})

	t.Run("delete removes the todo", func(t *testing.T) {
		todo := mustCreate(t, s, "temporary")
		if err := s.todos.Delete(ctx, todo.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.todos.Get(ctx, todo.ID); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("list is newest first and paged", func(t *testing.T) {
		var ids []string
		for _, title := range []string{"a", "b", "c"} {
			ids = append([]string{mustCreate(t, s, title).ID}, ids...) // newest first
		}
		// These three are the newest todos in the database, so they head the list.
		assertIDs(t, s, 3, 0, ids)
		assertIDs(t, s, 2, 1, ids[1:])
	})

	t.Run("activities are listed oldest first", func(t *testing.T) {
		todoID := uuid.NewString()
		want := []entity.Activity{
			mustNewActivity(t, todoID, entity.ActionCreated),
			mustNewActivity(t, todoID, entity.ActionRenamed),
			mustNewActivity(t, todoID, entity.ActionDeleted),
		}
		for _, a := range append(want, mustNewActivity(t, uuid.NewString(), entity.ActionCreated)) {
			if err := s.activities.Create(ctx, a); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.activities.ListByTodo(ctx, todoID)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("got %+v, %v; want %+v", got, err, want)
		}
		if got, err := s.activities.ListByTodo(ctx, uuid.NewString()); err != nil || len(got) != 0 {
			t.Errorf("unknown todo: got %v, %v; want empty", got, err)
		}
	})

	t.Run("transaction commits todo and activity together", func(t *testing.T) {
		todo := mustNewTodo(t, "committed")
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.todos.Create(ctx, todo); err != nil {
				return err
			}
			return s.activities.Create(ctx, mustNewActivity(t, todo.ID, entity.ActionCreated))
		})
		if err != nil {
			t.Fatal(err)
		}
		assertPersisted(t, s, todo.ID, true)
	})

	t.Run("transaction rolls back everything on error", func(t *testing.T) {
		todo := mustNewTodo(t, "rolled back")
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.todos.Create(ctx, todo); err != nil {
				return err
			}
			if err := s.activities.Create(ctx, mustNewActivity(t, todo.ID, entity.ActionCreated)); err != nil {
				return err
			}
			return errBoom
		})
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want errBoom", err)
		}
		assertPersisted(t, s, todo.ID, false)
	})
}

// testNow is truncated to milliseconds like the app's own clock, so values round-trip exactly.
func testNow() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func mustNewTodo(t *testing.T, title string) entity.Todo {
	t.Helper()
	todo, err := entity.NewTodo(title, testNow())
	if err != nil {
		t.Fatal(err)
	}
	return todo
}

func mustCreate(t *testing.T, s store, title string) entity.Todo {
	t.Helper()
	todo := mustNewTodo(t, title)
	if err := s.todos.Create(context.Background(), todo); err != nil {
		t.Fatal(err)
	}
	return todo
}

func mustNewActivity(t *testing.T, todoID string, action entity.Action) entity.Activity {
	t.Helper()
	activity, err := entity.NewActivity(todoID, action, testNow())
	if err != nil {
		t.Fatal(err)
	}
	return activity
}

func assertIDs(t *testing.T, s store, limit, offset int, want []string) {
	t.Helper()
	todos, err := s.todos.List(context.Background(), limit, offset)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, todo := range todos {
		got = append(got, todo.ID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("List(%d, %d) ids = %v, want %v", limit, offset, got, want)
	}
}

// assertPersisted checks that the todo and its one activity both exist, or both do not.
func assertPersisted(t *testing.T, s store, todoID string, want bool) {
	t.Helper()
	_, err := s.todos.Get(context.Background(), todoID)
	if found := err == nil; found != want {
		t.Errorf("todo persisted = %v (err %v), want %v", found, err, want)
	}
	activities, err := s.activities.ListByTodo(context.Background(), todoID)
	if err != nil {
		t.Fatal(err)
	}
	if found := len(activities) == 1; found != want {
		t.Errorf("activities = %d, want persisted = %v", len(activities), want)
	}
}
