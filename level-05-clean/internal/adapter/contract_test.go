//go:build integration

package adapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"

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
// cases ignore which database they run on. Subtests share one database, so they use fresh IDs.
func runContract(t *testing.T, s store) {
	ctx := context.Background()

	t.Run("create then find returns the same todo", func(t *testing.T) {
		todo := mustNewTodo(t, "write tests")
		if err := s.todos.Create(ctx, todo); err != nil {
			t.Fatal(err)
		}
		got, err := s.todos.FindByID(ctx, todo.ID)
		if err != nil || got != todo {
			t.Errorf("got %+v, %v; want %+v", got, err, todo)
		}
	})

	t.Run("unknown and malformed ids are not found", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			if _, err := s.todos.FindByID(ctx, id); !errors.Is(err, entity.ErrNotFound) {
				t.Errorf("FindByID(%q) err = %v, want ErrNotFound", id, err)
			}
			if err := s.todos.Delete(ctx, id); !errors.Is(err, entity.ErrNotFound) {
				t.Errorf("Delete(%q) err = %v, want ErrNotFound", id, err)
			}
		}
		if err := s.todos.Update(ctx, mustNewTodo(t, "ghost")); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("Update(unknown) err = %v, want ErrNotFound", err)
		}
	})

	t.Run("update persists every field", func(t *testing.T) {
		todo := mustNewTodo(t, "old")
		if err := s.todos.Create(ctx, todo); err != nil {
			t.Fatal(err)
		}
		if _, err := todo.Rename("new"); err != nil {
			t.Fatal(err)
		}
		todo.SetCompleted(true)
		if err := s.todos.Update(ctx, todo); err != nil {
			t.Fatal(err)
		}
		got, err := s.todos.FindByID(ctx, todo.ID)
		if err != nil || got != todo {
			t.Errorf("got %+v, %v; want %+v", got, err, todo)
		}
	})

	t.Run("delete removes the todo", func(t *testing.T) {
		todo := mustNewTodo(t, "temporary")
		if err := s.todos.Create(ctx, todo); err != nil {
			t.Fatal(err)
		}
		if err := s.todos.Delete(ctx, todo.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.todos.FindByID(ctx, todo.ID); !errors.Is(err, entity.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("list is newest first and paged", func(t *testing.T) {
		var ids []string
		for _, title := range []string{"a", "b", "c"} {
			todo := mustNewTodo(t, title)
			if err := s.todos.Create(ctx, todo); err != nil {
				t.Fatal(err)
			}
			ids = append([]string{todo.ID}, ids...) // newest first
		}
		// These three are the newest todos in the database, so they head the list.
		assertIDs(t, s, 3, 0, ids)
		assertIDs(t, s, 2, 1, ids[1:])
	})

	t.Run("activities are listed oldest first", func(t *testing.T) {
		todoID := uuid.NewString()
		want := []entity.Activity{
			entity.NewActivity(todoID, entity.ActionCreated),
			entity.NewActivity(todoID, entity.ActionRenamed),
			entity.NewActivity(todoID, entity.ActionDeleted),
		}
		for _, a := range append(want, entity.NewActivity(uuid.NewString(), entity.ActionCreated)) {
			if err := s.activities.Create(ctx, a); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.activities.ListByTodoID(ctx, todoID)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("got %+v, %v; want %+v", got, err, want)
		}
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			if got, err := s.activities.ListByTodoID(ctx, id); err != nil || len(got) != 0 {
				t.Errorf("ListByTodoID(%q) = %v, %v; want empty", id, got, err)
			}
		}
	})

	t.Run("transaction commits todo and activity together", func(t *testing.T) {
		todo := mustNewTodo(t, "committed")
		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := s.todos.Create(ctx, todo); err != nil {
				return err
			}
			return s.activities.Create(ctx, entity.NewActivity(todo.ID, entity.ActionCreated))
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
			if err := s.activities.Create(ctx, entity.NewActivity(todo.ID, entity.ActionCreated)); err != nil {
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

func mustNewTodo(t *testing.T, title string) entity.Todo {
	t.Helper()
	todo, err := entity.NewTodo(title)
	if err != nil {
		t.Fatal(err)
	}
	return todo
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
	_, err := s.todos.FindByID(context.Background(), todoID)
	if found := err == nil; found != want {
		t.Errorf("todo persisted = %v (err %v), want %v", found, err, want)
	}
	activities, err := s.activities.ListByTodoID(context.Background(), todoID)
	if err != nil {
		t.Fatal(err)
	}
	if found := len(activities) == 1; found != want {
		t.Errorf("activities = %d, want persisted = %v", len(activities), want)
	}
}
