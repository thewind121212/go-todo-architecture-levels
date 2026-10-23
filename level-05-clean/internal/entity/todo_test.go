package entity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/todo/level05/internal/entity"
)

var now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

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
			todo, err := entity.NewTodo(tt.title, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if todo.Title != tt.want || todo.Completed || todo.ID == "" || todo.CreatedAt != now || todo.UpdatedAt != now {
				t.Errorf("unexpected todo: %+v", todo)
			}
		})
	}
}

func TestTodoRename(t *testing.T) {
	todo, _ := entity.NewTodo("old", now)
	later := now.Add(time.Minute)

	if changed, err := todo.Rename(" old ", later); changed || err != nil || todo.UpdatedAt != now {
		t.Errorf("same title: changed = %v, err = %v, updated_at = %v", changed, err, todo.UpdatedAt)
	}
	if changed, err := todo.Rename("new", later); !changed || err != nil || todo.Title != "new" || todo.UpdatedAt != later {
		t.Errorf("new title: changed = %v, err = %v, todo = %+v", changed, err, todo)
	}
	if _, err := todo.Rename("", later); !errors.Is(err, entity.ErrInvalidTitle) {
		t.Errorf("blank title: err = %v, want ErrInvalidTitle", err)
	}
}

func TestTodoSetCompleted(t *testing.T) {
	todo, _ := entity.NewTodo("task", now)
	later := now.Add(time.Minute)

	if _, changed := todo.SetCompleted(false, later); changed || todo.UpdatedAt != now {
		t.Error("false -> false should not change anything")
	}
	if action, changed := todo.SetCompleted(true, later); !changed || action != entity.ActionCompleted || todo.UpdatedAt != later {
		t.Errorf("false -> true: got %q, %v, %+v", action, changed, todo)
	}
	if action, changed := todo.SetCompleted(false, later); !changed || action != entity.ActionReopened {
		t.Errorf("true -> false: got %q, %v", action, changed)
	}
}
