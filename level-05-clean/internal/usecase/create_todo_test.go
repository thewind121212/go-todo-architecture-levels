package usecase_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

func TestCreateTodo(t *testing.T) {
	t.Run("saves the todo and a created activity", func(t *testing.T) {
		todos, activities, tx := newFakeTodos(), &fakeActivities{}, &fakeTx{}
		uc := usecase.NewCreateTodo(todos, activities, tx)

		out, err := uc.Execute(context.Background(), usecase.CreateTodoInput{Title: "  buy milk "})
		if err != nil {
			t.Fatal(err)
		}
		if out.Title != "buy milk" || out.Completed || out.ID == "" {
			t.Errorf("unexpected output: %+v", out)
		}
		if _, ok := todos.items[out.ID]; !ok {
			t.Error("todo was not saved")
		}
		if got := activities.actions(); !slices.Equal(got, []entity.Action{entity.ActionCreated}) {
			t.Errorf("activities = %v, want [created]", got)
		}
		if activities.items[0].CreatedAt != out.CreatedAt {
			t.Errorf("activity created_at %v != todo created_at %v", activities.items[0].CreatedAt, out.CreatedAt)
		}
		if tx.calls != 1 || tx.failed {
			t.Errorf("tx calls = %d, failed = %v; want 1, false", tx.calls, tx.failed)
		}
	})

	t.Run("invalid title never starts a transaction", func(t *testing.T) {
		tx := &fakeTx{}
		uc := usecase.NewCreateTodo(newFakeTodos(), &fakeActivities{}, tx)

		_, err := uc.Execute(context.Background(), usecase.CreateTodoInput{Title: " "})
		if !errors.Is(err, entity.ErrInvalidTitle) {
			t.Errorf("err = %v, want ErrInvalidTitle", err)
		}
		if tx.calls != 0 {
			t.Errorf("tx calls = %d, want 0", tx.calls)
		}
	})

	t.Run("activity failure fails the transaction", func(t *testing.T) {
		tx := &fakeTx{}
		uc := usecase.NewCreateTodo(newFakeTodos(), &fakeActivities{err: errBoom}, tx)

		_, err := uc.Execute(context.Background(), usecase.CreateTodoInput{Title: "task"})
		if !errors.Is(err, errBoom) {
			t.Errorf("err = %v, want errBoom", err)
		}
		if !tx.failed {
			t.Error("tx should be reported failed so it rolls back")
		}
	})
}
