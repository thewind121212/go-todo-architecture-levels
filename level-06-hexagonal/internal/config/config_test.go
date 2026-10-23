package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"example.com/todo/level06/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("HTTP_ADDR", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, config.DriverPostgres, cfg.DBDriver)
	require.Equal(t, ":8086", cfg.HTTPAddr)
	require.Equal(t, "todo_l06", cfg.MongoDB)
}

func TestLoadRejectsUnknownDriver(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")

	_, err := config.Load()
	require.Error(t, err)
}
