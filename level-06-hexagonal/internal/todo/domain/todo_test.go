package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level06/internal/todo/domain"
)

func TestNewTodo(t *testing.T) {
	todo, err := domain.NewTodo("  buy milk  ")
	require.NoError(t, err)

	require.Equal(t, "buy milk", todo.Title)
	require.False(t, todo.Completed)
	require.Equal(t, todo.CreatedAt, todo.UpdatedAt)
	require.Equal(t, time.UTC, todo.CreatedAt.Location())
	require.Equal(t, todo.CreatedAt, todo.CreatedAt.Truncate(time.Millisecond))
	require.Equal(t, uuid.Version(7), uuid.MustParse(todo.ID).Version())
}

func TestNewTodoTitleRules(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"empty", "", true},
		{"only spaces", "   ", true},
		{"200 multi-byte runes", strings.Repeat("é", 200), false},
		{"201 runes", strings.Repeat("x", 201), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewTodo(tt.title)
			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrInvalidTitle)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRename(t *testing.T) {
	todo, err := domain.NewTodo("a")
	require.NoError(t, err)

	changed, err := todo.Rename(" a ")
	require.NoError(t, err)
	require.False(t, changed, "same title after trimming is not a change")

	changed, err = todo.Rename("b")
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "b", todo.Title)

	_, err = todo.Rename(" ")
	require.ErrorIs(t, err, domain.ErrInvalidTitle)
	require.Equal(t, "b", todo.Title)
}

func TestSetCompleted(t *testing.T) {
	todo, err := domain.NewTodo("a")
	require.NoError(t, err)

	require.False(t, todo.SetCompleted(false))
	require.True(t, todo.SetCompleted(true))
	require.True(t, todo.Completed)
	require.Equal(t, domain.ActionCompleted, domain.CompletionAction(true))
	require.Equal(t, domain.ActionReopened, domain.CompletionAction(false))
}

func TestIsValidID(t *testing.T) {
	id := uuid.Must(uuid.NewV7()).String()
	require.True(t, domain.IsValidID(id))
	require.False(t, domain.IsValidID("not-a-uuid"))
	require.False(t, domain.IsValidID(strings.ToUpper(id)))
	require.False(t, domain.IsValidID("{"+id+"}"))
}
