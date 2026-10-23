package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"example.com/todo/level07/internal/todo/domain"
)

func TestNewTitleRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{"trims spaces", "  buy milk  ", "buy milk", nil},
		{"empty", "", "", domain.ErrInvalidTitle},
		{"only spaces", "   ", "", domain.ErrInvalidTitle},
		{"200 multi-byte runes", strings.Repeat("é", 200), strings.Repeat("é", 200), nil},
		{"201 runes", strings.Repeat("x", 201), "", domain.ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, err := domain.NewTitle(tt.raw)
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, title.String())
		})
	}
}

func TestTitlesWithTheSameValueAreEqual(t *testing.T) {
	a, err := domain.NewTitle(" same ")
	require.NoError(t, err)
	b, err := domain.NewTitle("same")
	require.NoError(t, err)
	require.True(t, a == b, "value objects compare by value")
}
