package usecase_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

func TestUpdateTodo(t *testing.T) {
	t.Run("bad input is rejected before the transaction, even for an unknown id", func(t *testing.T) {
		tests := []struct {
			name    string
			in      usecase.UpdateTodoInput
			wantErr error
		}{
			{name: "empty patch", in: usecase.UpdateTodoInput{ID: "missing"}, wantErr: entity.ErrEmptyPatch},
			{name: "blank title", in: usecase.UpdateTodoInput{ID: "missing", Title: new(" ")}, wantErr: entity.ErrInvalidTitle},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tx := &fakeTx{}
				uc := usecase.NewUpdateTodo(newFakeTodos(), &fakeActivities{}, tx)

				_, err := uc.Execute(context.Background(), tt.in)
				if !errors.Is(err, tt.wantErr) || tx.calls != 0 {
					t.Errorf("err = %v, tx calls = %d; want %v, 0", err, tx.calls, tt.wantErr)
				}
			})
		}
	})

	t.Run("unknown todo", func(t *testing.T) {
		tx := &fakeTx{}
		uc := usecase.NewUpdateTodo(newFakeTodos(), &fakeActivities{}, tx)

		_, err := uc.Execute(context.Background(), usecase.UpdateTodoInput{ID: "missing", Completed: new(true)})
		if !errors.Is(err, entity.ErrNotFound) || !tx.failed {
			t.Errorf("err = %v, tx failed = %v; want ErrNotFound, true", err, tx.failed)
		}
	})

	t.Run("title and completed write two activities in order", func(t *testing.T) {
		todo := mustNewTodo("old")
		todos, activities, tx := newFakeTodos(todo), &fakeActivities{}, &fakeTx{}
		uc := usecase.NewUpdateTodo(todos, activities, tx)

		out, err := uc.Execute(context.Background(), usecase.UpdateTodoInput{ID: todo.ID, Title: new(" new "), Completed: new(true)})
		if err != nil {
			t.Fatal(err)
		}
		if out.Title != "new" || !out.Completed || !out.UpdatedAt.After(out.CreatedAt) {
			t.Errorf("unexpected output: %+v", out)
		}
		if saved := todos.items[todo.ID]; saved.Title != "new" || !saved.Completed {
			t.Errorf("unexpected saved todo: %+v", saved)
		}
		if todos.locked != 1 {
			t.Errorf("GetForUpdate calls = %d, want 1", todos.locked)
		}
		want := []entity.Action{entity.ActionRenamed, entity.ActionCompleted}
		if got := activities.actions(); !slices.Equal(got, want) {
			t.Errorf("activities = %v, want %v", got, want)
		}
		for _, a := range activities.items {
			if a.CreatedAt != out.UpdatedAt {
				t.Errorf("activity created_at %v != todo updated_at %v", a.CreatedAt, out.UpdatedAt)
			}
		}
	})

	t.Run("no real change writes nothing", func(t *testing.T) {
		todo := mustNewTodo("same")
		todos, activities := newFakeTodos(todo), &fakeActivities{}
		uc := usecase.NewUpdateTodo(todos, activities, &fakeTx{})

		out, err := uc.Execute(context.Background(), usecase.UpdateTodoInput{ID: todo.ID, Title: new(" same "), Completed: new(false)})
		if err != nil || out.Title != "same" || out.UpdatedAt != todo.UpdatedAt {
			t.Fatalf("got %+v, %v", out, err)
		}
		if todos.updates != 0 || len(activities.items) != 0 {
			t.Errorf("updates = %d, activities = %d; want 0, 0", todos.updates, len(activities.items))
		}
	})

	t.Run("activity failure fails the transaction", func(t *testing.T) {
		todo := mustNewTodo("task")
		tx := &fakeTx{}
		uc := usecase.NewUpdateTodo(newFakeTodos(todo), &fakeActivities{err: errBoom}, tx)

		_, err := uc.Execute(context.Background(), usecase.UpdateTodoInput{ID: todo.ID, Completed: new(true)})
		if !errors.Is(err, errBoom) || !tx.failed {
			t.Errorf("err = %v, tx failed = %v; want errBoom, true", err, tx.failed)
		}
	})
}
