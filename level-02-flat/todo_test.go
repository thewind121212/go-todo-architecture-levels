// todo_test.go: table tests for the domain rules in todo.go.

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
		{name: "plain", in: "buy milk", want: "buy milk"},
		{name: "trimmed", in: "  buy milk \n", want: "buy milk"},
		{name: "empty", in: "", wantErr: ErrInvalidTitle},
		{name: "only spaces", in: "   ", wantErr: ErrInvalidTitle},
		{name: "200 runes", in: strings.Repeat("é", 200), want: strings.Repeat("é", 200)},
		{name: "201 runes", in: strings.Repeat("é", 201), wantErr: ErrInvalidTitle},
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
		in        Patch
		wantTitle string
		wantErr   error
	}{
		{name: "empty", in: Patch{}, wantErr: ErrEmptyPatch},
		{name: "blank title", in: Patch{Title: new("  ")}, wantErr: ErrInvalidTitle},
		{name: "title is trimmed", in: Patch{Title: new(" new ")}, wantTitle: "new"},
		{name: "completed only", in: Patch{Completed: new(true)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validatePatch(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.Title != nil && *got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", *got.Title, tt.wantTitle)
			}
		})
	}
}

func TestApplyPatch(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := created.Add(time.Hour)
	tests := []struct {
		name          string
		completed     bool
		patch         Patch
		wantTitle     string
		wantCompleted bool
		wantActions   []Action
	}{
		{name: "same title", patch: Patch{Title: new("a")}, wantTitle: "a"},
		{name: "same completed", patch: Patch{Completed: new(false)}, wantTitle: "a"},
		{name: "rename", patch: Patch{Title: new("b")}, wantTitle: "b", wantActions: []Action{ActionRenamed}},
		{name: "complete", patch: Patch{Completed: new(true)}, wantTitle: "a", wantCompleted: true,
			wantActions: []Action{ActionCompleted}},
		{name: "reopen", completed: true, patch: Patch{Completed: new(false)}, wantTitle: "a",
			wantActions: []Action{ActionReopened}},
		{name: "rename and complete", patch: Patch{Title: new("b"), Completed: new(true)}, wantTitle: "b",
			wantCompleted: true, wantActions: []Action{ActionRenamed, ActionCompleted}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := Todo{ID: newID(), Title: "a", Completed: tt.completed, CreatedAt: created, UpdatedAt: created}
			activities := applyPatch(&todo, tt.patch, now)

			if todo.Title != tt.wantTitle || todo.Completed != tt.wantCompleted {
				t.Errorf("todo = %q/%v, want %q/%v", todo.Title, todo.Completed, tt.wantTitle, tt.wantCompleted)
			}
			var actions []Action
			for _, a := range activities {
				actions = append(actions, a.Action)
				if a.TodoID != todo.ID || !a.CreatedAt.Equal(now) {
					t.Errorf("activity = %+v, want todo %s at %v", a, todo.ID, now)
				}
			}
			if !slices.Equal(actions, tt.wantActions) {
				t.Errorf("actions = %v, want %v", actions, tt.wantActions)
			}
			wantUpdated := created
			if len(tt.wantActions) > 0 {
				wantUpdated = now
			}
			if !todo.UpdatedAt.Equal(wantUpdated) {
				t.Errorf("updated_at = %v, want %v", todo.UpdatedAt, wantUpdated)
			}
		})
	}
}
