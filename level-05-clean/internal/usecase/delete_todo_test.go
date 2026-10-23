package usecase_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

func TestDeleteTodo(t *testing.T) {
	t.Run("deletes the todo and records a deleted activity", func(t *testing.T) {
		todo := mustNewTodo("task")
		todos, activities := newFakeTodos(todo), &fakeActivities{}
		uc := usecase.NewDeleteTodo(todos, activities, &fakeTx{})

		if err := uc.Execute(context.Background(), usecase.DeleteTodoInput{ID: todo.ID}); err != nil {
			t.Fatal(err)
		}
		if _, ok := todos.items[todo.ID]; ok {
			t.Error("todo still exists")
		}
		if got := activities.actions(); !slices.Equal(got, []entity.Action{entity.ActionDeleted}) {
			t.Errorf("activities = %v, want [deleted]", got)
		}
	})

	t.Run("unknown todo records nothing", func(t *testing.T) {
		activities := &fakeActivities{}
		uc := usecase.NewDeleteTodo(newFakeTodos(), activities, &fakeTx{})

		err := uc.Execute(context.Background(), usecase.DeleteTodoInput{ID: "missing"})
		if !errors.Is(err, entity.ErrNotFound) || len(activities.items) != 0 {
			t.Errorf("err = %v, activities = %d; want ErrNotFound, 0", err, len(activities.items))
		}
	})

	t.Run("activity failure fails the transaction", func(t *testing.T) {
		todo := mustNewTodo("task")
		tx := &fakeTx{}
		uc := usecase.NewDeleteTodo(newFakeTodos(todo), &fakeActivities{err: errBoom}, tx)

		err := uc.Execute(context.Background(), usecase.DeleteTodoInput{ID: todo.ID})
		if !errors.Is(err, errBoom) || !tx.failed {
			t.Errorf("err = %v, tx failed = %v; want errBoom, true", err, tx.failed)
		}
	})
}
