package usecase_test

import (
	"context"
	"testing"

	"example.com/todo/level05/internal/usecase"
)

func TestListTodos(t *testing.T) {
	t.Run("passes the page to the repository", func(t *testing.T) {
		todos := newFakeTodos(mustNewTodo("a"), mustNewTodo("b"))
		uc := usecase.NewListTodos(todos)

		out, err := uc.Execute(context.Background(), usecase.ListTodosInput{Limit: 5, Offset: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Items) != 2 || out.Limit != 5 || out.Offset != 10 {
			t.Errorf("unexpected output: %+v", out)
		}
		if todos.listLimit != 5 || todos.listOffset != 10 {
			t.Errorf("repository got limit %d offset %d", todos.listLimit, todos.listOffset)
		}
	})

	t.Run("empty page has non-nil items", func(t *testing.T) {
		out, err := usecase.NewListTodos(newFakeTodos()).Execute(context.Background(), usecase.ListTodosInput{Limit: 20})
		if err != nil || out.Items == nil {
			t.Errorf("items = %#v, err = %v; want empty slice", out.Items, err)
		}
	})
}
