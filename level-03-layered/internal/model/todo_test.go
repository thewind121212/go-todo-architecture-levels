package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	created = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later   = created.Add(time.Hour)
)

func TestNewTodo(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		want    string
		wantErr error
	}{
		{name: "trims spaces", title: "  buy milk  ", want: "buy milk"},
		{name: "200 runes is ok", title: strings.Repeat("é", 200), want: strings.Repeat("é", 200)},
		{name: "blank", title: "   ", wantErr: ErrInvalidTitle},
		{name: "201 runes", title: strings.Repeat("x", 201), wantErr: ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTodo(tt.title, created)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Title != tt.want || got.Completed || got.ID == "" {
				t.Errorf("got %+v, want title %q, not completed, with id", got, tt.want)
			}
			if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(created) {
				t.Errorf("timestamps = %v / %v, want %v", got.CreatedAt, got.UpdatedAt, created)
			}
		})
	}
}

func TestTodoApply(t *testing.T) {
	tests := []struct {
		name        string
		start       Todo
		patch       Patch
		wantActions []Action
		wantErr     error
	}{
		{name: "rename", patch: Patch{Title: new("new")}, wantActions: []Action{ActionRenamed}},
		{name: "same title after trim", patch: Patch{Title: new(" old ")}},
		{name: "complete", patch: Patch{Completed: new(true)}, wantActions: []Action{ActionCompleted}},
		{name: "reopen", start: Todo{Completed: true}, patch: Patch{Completed: new(false)}, wantActions: []Action{ActionReopened}},
		{name: "already completed", start: Todo{Completed: true}, patch: Patch{Completed: new(true)}},
		{
			name:        "rename and complete",
			patch:       Patch{Title: new("new"), Completed: new(true)},
			wantActions: []Action{ActionRenamed, ActionCompleted},
		},
		{name: "empty patch", patch: Patch{}, wantErr: ErrEmptyPatch},
		{name: "invalid title", patch: Patch{Title: new(" ")}, wantErr: ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := tt.start
			todo.Title = "old"
			todo.UpdatedAt = created

			actions, err := todo.Apply(tt.patch, later)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(actions, tt.wantActions) {
				t.Errorf("actions = %v, want %v", actions, tt.wantActions)
			}
			wantUpdated := created
			if len(tt.wantActions) > 0 {
				wantUpdated = later
			}
			if !todo.UpdatedAt.Equal(wantUpdated) {
				t.Errorf("updated_at = %v, want %v", todo.UpdatedAt, wantUpdated)
			}
		})
	}
}
