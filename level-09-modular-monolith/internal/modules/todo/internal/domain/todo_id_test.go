package domain_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/todo/internal/domain"
)

func TestNewTodoIDIsUUIDv7(t *testing.T) {
	id := domain.NewTodoID()
	require.Equal(t, uuid.Version(7), uuid.MustParse(id.String()).Version())
	require.NotEqual(t, id, domain.NewTodoID())
}

func TestParseTodoIDKeepsTheCanonicalForm(t *testing.T) {
	raw := uuid.NewString()

	id, err := domain.ParseTodoID(strings.ToUpper(raw))

	require.NoError(t, err)
	require.Equal(t, raw, id.String())
	again, err := domain.ParseTodoID(raw)
	require.NoError(t, err)
	require.True(t, id == again, "value objects compare by value")
}

func TestParseTodoIDRejectsMalformedIDs(t *testing.T) {
	for _, raw := range []string{"", "not-a-uuid", "1234"} {
		_, err := domain.ParseTodoID(raw)
		require.ErrorIs(t, err, domain.ErrInvalidID, raw)
	}
}
