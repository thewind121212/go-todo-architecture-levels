package todo

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

func TestNew(t *testing.T) {
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
			got, err := New(tt.title, created)
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

func TestValidatePatch(t *testing.T) {
	tests := []struct {
		name      string
		patch     Patch
		wantTitle string
		wantErr   error
	}{
		{name: "trims title", patch: Patch{Title: new(" new ")}, wantTitle: "new"},
		{name: "completed only", patch: Patch{Completed: new(true)}},
		{name: "empty patch", patch: Patch{}, wantErr: ErrEmptyPatch},
		{name: "blank title", patch: Patch{Title: new(" ")}, wantErr: ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePatch(tt.patch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.Title != nil && *got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", *got.Title, tt.wantTitle)
			}
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name        string
		start       Todo
		patch       Patch
		wantActions []Action
	}{
		{name: "rename", patch: Patch{Title: new("new")}, wantActions: []Action{ActionRenamed}},
		{name: "same title", patch: Patch{Title: new("old")}},
		{name: "complete", patch: Patch{Completed: new(true)}, wantActions: []Action{ActionCompleted}},
		{name: "reopen", start: Todo{Completed: true}, patch: Patch{Completed: new(false)}, wantActions: []Action{ActionReopened}},
		{name: "already completed", start: Todo{Completed: true}, patch: Patch{Completed: new(true)}},
		{
			name:        "rename and complete",
			patch:       Patch{Title: new("new"), Completed: new(true)},
			wantActions: []Action{ActionRenamed, ActionCompleted},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			todo := tt.start
			todo.Title = "old"
			todo.UpdatedAt = created

			actions := todo.Apply(tt.patch, later)
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
