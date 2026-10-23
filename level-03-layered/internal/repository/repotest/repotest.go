// Package repotest is the contract suite every storage adapter must pass. The
// postgres and mongo integration tests both call Run, so the two adapters are
// checked against exactly the same expectations.
package repotest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"example.com/todo/level03/internal/model"
	"example.com/todo/level03/internal/repository"
)

// Storage is one adapter's implementation of the repository interfaces.
type Storage struct {
	Tx         repository.TxManager
	Todos      repository.TodoRepository
	Activities repository.ActivityRepository
}

// Run runs every contract test. newStorage is called once per test and must
// return storage backed by empty collections.
func Run(t *testing.T, newStorage func(t *testing.T) Storage) {
	tests := []struct {
		name string
		fn   func(t *testing.T, s Storage)
	}{
		{"CreateAndGet", testCreateAndGet},
		{"GetUnknown", testGetUnknown},
		{"ListNewestFirst", testListNewestFirst},
		{"Update", testUpdate},
		{"Delete", testDelete},
		{"Activities", testActivities},
		{"TxCommit", testTxCommit},
		{"TxRollback", testTxRollback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, newStorage(t))
		})
	}
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func newTodo(t *testing.T, title string) model.Todo {
	t.Helper()
	todo, err := model.NewTodo(title, now())
	if err != nil {
		t.Fatalf("new todo: %v", err)
	}
	return todo
}

func newActivity(t *testing.T, todoID string, action model.Action) model.Activity {
	t.Helper()
	a, err := model.NewActivity(todoID, action, now())
	if err != nil {
		t.Fatalf("new activity: %v", err)
	}
	return a
}

func mustCreate(t *testing.T, s Storage, todo model.Todo) {
	t.Helper()
	if err := s.Todos.Create(t.Context(), todo); err != nil {
		t.Fatalf("create todo: %v", err)
	}
}

func assertNotFound(t *testing.T, s Storage, id string) {
	t.Helper()
	if _, err := s.Todos.Get(t.Context(), id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("get %s: err = %v, want ErrNotFound", id, err)
	}
}

func testCreateAndGet(t *testing.T, s Storage) {
	todo := newTodo(t, "buy milk")
	mustCreate(t, s, todo)

	got, err := s.Todos.Get(t.Context(), todo.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != todo {
		t.Errorf("got %+v, want %+v", got, todo)
	}
}

func testGetUnknown(t *testing.T, s Storage) {
	assertNotFound(t, s, uuid.NewString())
}

func testListNewestFirst(t *testing.T, s Storage) {
	var ids []string
	for _, title := range []string{"a", "b", "c"} {
		todo := newTodo(t, title)
		mustCreate(t, s, todo)
		ids = append(ids, todo.ID)
	}
	slices.Reverse(ids) // newest first

	tests := []struct {
		limit, offset int
		want          []string
	}{
		{limit: 10, offset: 0, want: ids},
		{limit: 2, offset: 0, want: ids[:2]},
		{limit: 2, offset: 2, want: ids[2:]},
		{limit: 2, offset: 5, want: nil},
	}
	for _, tt := range tests {
		todos, err := s.Todos.List(t.Context(), tt.limit, tt.offset)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var got []string
		for _, todo := range todos {
			got = append(got, todo.ID)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("List(%d, %d) = %v, want %v", tt.limit, tt.offset, got, tt.want)
		}
	}
}

func testUpdate(t *testing.T, s Storage) {
	todo := newTodo(t, "old")
	mustCreate(t, s, todo)

	todo.Title = "new"
	todo.Completed = true
	todo.UpdatedAt = now().Add(time.Second)
	if err := s.Todos.Update(t.Context(), todo); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.Todos.Get(t.Context(), todo.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != todo {
		t.Errorf("got %+v, want %+v", got, todo)
	}

	// completed=false is a zero value; it must still be written.
	todo.Completed = false
	if err := s.Todos.Update(t.Context(), todo); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := s.Todos.Get(t.Context(), todo.ID); got.Completed {
		t.Error("completed = true after updating it to false")
	}

	unknown := newTodo(t, "never stored")
	if err := s.Todos.Update(t.Context(), unknown); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("update unknown: err = %v, want ErrNotFound", err)
	}
}

func testDelete(t *testing.T, s Storage) {
	todo := newTodo(t, "title")
	mustCreate(t, s, todo)

	if err := s.Todos.Delete(t.Context(), todo.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	assertNotFound(t, s, todo.ID)
	if err := s.Todos.Delete(t.Context(), todo.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete again: err = %v, want ErrNotFound", err)
	}
}

func testActivities(t *testing.T, s Storage) {
	todoID, otherID := uuid.NewString(), uuid.NewString()
	want := []model.Activity{
		newActivity(t, todoID, model.ActionCreated),
		newActivity(t, todoID, model.ActionRenamed),
	}
	for _, a := range []model.Activity{want[0], newActivity(t, otherID, model.ActionCreated), want[1]} {
		if err := s.Activities.Create(t.Context(), a); err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	got, err := s.Activities.ListByTodo(t.Context(), todoID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	got, err = s.Activities.ListByTodo(t.Context(), uuid.NewString())
	if err != nil || len(got) != 0 {
		t.Errorf("unknown todo: got %v, %v; want empty", got, err)
	}
}

func testTxCommit(t *testing.T, s Storage) {
	todo := newTodo(t, "committed")
	err := s.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.Todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.Activities.Create(ctx, newActivity(t, todo.ID, model.ActionCreated))
	})
	if err != nil {
		t.Fatalf("within tx: %v", err)
	}

	if _, err := s.Todos.Get(t.Context(), todo.ID); err != nil {
		t.Errorf("get after commit: %v", err)
	}
	if acts, _ := s.Activities.ListByTodo(t.Context(), todo.ID); len(acts) != 1 {
		t.Errorf("activities after commit = %d, want 1", len(acts))
	}
}

// testTxRollback is the core lesson: writes made through the repositories
// with the tx ctx disappear when fn returns an error.
func testTxRollback(t *testing.T, s Storage) {
	errBoom := errors.New("boom")
	todo := newTodo(t, "rolled back")
	err := s.Tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.Todos.Create(ctx, todo); err != nil {
			return err
		}
		if err := s.Activities.Create(ctx, newActivity(t, todo.ID, model.ActionCreated)); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("within tx: err = %v, want %v", err, errBoom)
	}

	assertNotFound(t, s, todo.ID)
	acts, err := s.Activities.ListByTodo(t.Context(), todo.ID)
	if err != nil || len(acts) != 0 {
		t.Errorf("activities after rollback = %v, %v; want none", acts, err)
	}
}
