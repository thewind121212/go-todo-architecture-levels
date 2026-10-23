package usecase_test

import (
	"context"
	"errors"
	"testing"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

func TestGetTodo(t *testing.T) {
	todo := mustNewTodo("task")
	uc := usecase.NewGetTodo(newFakeTodos(todo))

	out, err := uc.Execute(context.Background(), usecase.GetTodoInput{ID: todo.ID})
	if err != nil || out.ID != todo.ID || out.Title != "task" {
		t.Errorf("got %+v, %v", out, err)
	}

	_, err = uc.Execute(context.Background(), usecase.GetTodoInput{ID: "missing"})
	if !errors.Is(err, entity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
