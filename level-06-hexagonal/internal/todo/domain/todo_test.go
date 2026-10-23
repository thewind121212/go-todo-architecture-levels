package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level06/internal/todo/domain"
)

var now = time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC)

func TestNewTodo(t *testing.T) {
	todo, err := domain.NewTodo("  buy milk  ", now)
	require.NoError(t, err)

	require.Equal(t, "buy milk", todo.Title)
	require.False(t, todo.Completed)
	require.Equal(t, now, todo.CreatedAt)
	require.Equal(t, now, todo.UpdatedAt)
	require.Equal(t, uuid.Version(7), uuid.MustParse(todo.ID).Version())
}

func TestTitleRules(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{"empty", "", domain.ErrInvalidTitle},
		{"only spaces", "   ", domain.ErrInvalidTitle},
		{"200 multi-byte runes", strings.Repeat("é", 200), nil},
		{"201 runes", strings.Repeat("x", 201), domain.ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewTodo(tt.title, now)
			require.ErrorIs(t, err, tt.wantErr)
			_, err = domain.ValidateTitle(tt.title)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestTodoRename(t *testing.T) {
	todo, err := domain.NewTodo("old", now)
	require.NoError(t, err)
	later := now.Add(time.Second)

	changed, err := todo.Rename(" old ", later)
	require.NoError(t, err)
	require.False(t, changed, "same title after trimming is not a change")
	require.Equal(t, now, todo.UpdatedAt)

	changed, err = todo.Rename("new", later)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "new", todo.Title)
	require.Equal(t, later, todo.UpdatedAt)

	_, err = todo.Rename(" ", later)
	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestTodoSetCompleted(t *testing.T) {
	todo, err := domain.NewTodo("task", now)
	require.NoError(t, err)

	_, changed := todo.SetCompleted(false, now)
	require.False(t, changed, "false -> false is not a change")

	action, changed := todo.SetCompleted(true, now)
	require.True(t, changed)
	require.Equal(t, domain.ActionCompleted, action)

	action, changed = todo.SetCompleted(false, now)
	require.True(t, changed)
	require.Equal(t, domain.ActionReopened, action)
}
