package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"plain", "buy milk", "buy milk", nil},
		{"trimmed", "  buy milk \n", "buy milk", nil},
		{"empty", "", "", ErrInvalidTitle},
		{"blank", "   ", "", ErrInvalidTitle},
		{"200 runes", strings.Repeat("é", 200), strings.Repeat("é", 200), nil},
		{"201 runes", strings.Repeat("é", 201), "", ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateTitle(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("title = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePatch(t *testing.T) {
	tests := []struct {
		name      string
		in        todoPatch
		wantTitle *string
		wantErr   error
	}{
		{"empty", todoPatch{}, nil, ErrEmptyPatch},
		{"completed only", todoPatch{Completed: new(true)}, nil, nil},
		{"title trimmed", todoPatch{Title: new(" new ")}, new("new"), nil},
		{"blank title", todoPatch{Title: new(" "), Completed: new(true)}, nil, ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validatePatch(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantTitle != nil && (got.Title == nil || *got.Title != *tt.wantTitle) {
				t.Errorf("title = %v, want %q", got.Title, *tt.wantTitle)
			}
		})
	}
}

func TestNewTodo(t *testing.T) {
	todo, err := newTodo("  write tests ")
	if err != nil {
		t.Fatal(err)
	}
	if todo.ID == "" || todo.Title != "write tests" || todo.Completed {
		t.Errorf("unexpected todo %+v", todo)
	}
	if todo.CreatedAt.Location() != time.UTC || !todo.CreatedAt.Equal(todo.UpdatedAt) {
		t.Errorf("timestamps must be equal and UTC: %+v", todo)
	}

	if _, err := newTodo(" "); !errors.Is(err, ErrInvalidTitle) {
		t.Errorf("err = %v, want %v", err, ErrInvalidTitle)
	}
}

func TestApplyPatch(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := created.Add(time.Hour)

	tests := []struct {
		name        string
		start       Todo
		patch       todoPatch
		wantTitle   string
		wantDone    bool
		wantActions []string
	}{
		{"rename", Todo{Title: "a"}, todoPatch{Title: new("b")}, "b", false, []string{ActionRenamed}},
		{"same title", Todo{Title: "a"}, todoPatch{Title: new("a")}, "a", false, nil},
		{"complete", Todo{Title: "a"}, todoPatch{Completed: new(true)}, "a", true, []string{ActionCompleted}},
		{"reopen", Todo{Title: "a", Completed: true}, todoPatch{Completed: new(false)}, "a", false, []string{ActionReopened}},
		{"already completed", Todo{Title: "a", Completed: true}, todoPatch{Completed: new(true)}, "a", true, nil},
		{
			"rename and complete", Todo{Title: "a"}, todoPatch{Title: new("b"), Completed: new(true)},
			"b", true, []string{ActionRenamed, ActionCompleted},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := tt.start
			todo.ID = "todo-1"
			todo.UpdatedAt = created

			activities := applyPatch(&todo, tt.patch, at)

			if todo.Title != tt.wantTitle || todo.Completed != tt.wantDone {
				t.Errorf("todo = %+v, want title %q completed %v", todo, tt.wantTitle, tt.wantDone)
			}
			var actions []string
			for _, a := range activities {
				if a.TodoID != "todo-1" || !a.CreatedAt.Equal(at) || a.ID == "" {
					t.Errorf("bad activity %+v", a)
				}
				actions = append(actions, a.Action)
			}
			if !slices.Equal(actions, tt.wantActions) {
				t.Errorf("actions = %v, want %v", actions, tt.wantActions)
			}
			wantUpdated := created
			if len(tt.wantActions) > 0 {
				wantUpdated = at
			}
			if !todo.UpdatedAt.Equal(wantUpdated) {
				t.Errorf("updated_at = %v, want %v", todo.UpdatedAt, wantUpdated)
			}
		})
	}
}
