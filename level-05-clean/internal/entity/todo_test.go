package entity_test

import (
	"errors"
	"strings"
	"testing"

	"example.com/todo/level05/internal/entity"
)

func TestNewTodo(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		want    string
		wantErr error
	}{
		{name: "trims spaces", title: "  buy milk  ", want: "buy milk"},
		{name: "200 runes is the limit", title: strings.Repeat("é", 200), want: strings.Repeat("é", 200)},
		{name: "blank", title: "   ", wantErr: entity.ErrInvalidTitle},
		{name: "201 runes", title: strings.Repeat("x", 201), wantErr: entity.ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo, err := entity.NewTodo(tt.title)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if todo.Title != tt.want || todo.Completed || todo.ID == "" || todo.CreatedAt.IsZero() {
				t.Errorf("unexpected todo: %+v", todo)
			}
		})
	}
}

func TestTodoRename(t *testing.T) {
	todo, _ := entity.NewTodo("old")

	if changed, err := todo.Rename(" old "); changed || err != nil {
		t.Errorf("same title: changed = %v, err = %v; want false, nil", changed, err)
	}
	if changed, err := todo.Rename("new"); !changed || err != nil || todo.Title != "new" {
		t.Errorf("new title: changed = %v, err = %v, title = %q", changed, err, todo.Title)
	}
	if _, err := todo.Rename(""); !errors.Is(err, entity.ErrInvalidTitle) {
		t.Errorf("blank title: err = %v, want ErrInvalidTitle", err)
	}
}

func TestTodoSetCompleted(t *testing.T) {
	todo, _ := entity.NewTodo("task")

	if _, changed := todo.SetCompleted(false); changed {
		t.Error("false -> false should not change")
	}
	if action, changed := todo.SetCompleted(true); !changed || action != entity.ActionCompleted {
		t.Errorf("false -> true: got %q, %v", action, changed)
	}
	if action, changed := todo.SetCompleted(false); !changed || action != entity.ActionReopened {
		t.Errorf("true -> false: got %q, %v", action, changed)
	}
}
