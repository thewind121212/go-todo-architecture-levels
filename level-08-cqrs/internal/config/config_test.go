package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("OUTBOX_POLL_INTERVAL", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, config.DriverPostgres, cfg.DBDriver)
	require.Equal(t, ":8088", cfg.HTTPAddr)
	require.Equal(t, "todo_l08", cfg.MongoDB)
	require.Equal(t, 100*time.Millisecond, cfg.OutboxPollInterval)
}

func TestLoadReadsThePollInterval(t *testing.T) {
	t.Setenv("OUTBOX_POLL_INTERVAL", "2s")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, 2*time.Second, cfg.OutboxPollInterval)
}

func TestLoadRejectsABadPollInterval(t *testing.T) {
	for _, raw := range []string{"soon", "0s", "-1s"} {
		t.Setenv("OUTBOX_POLL_INTERVAL", raw)

		_, err := config.Load()
		require.Error(t, err, raw)
	}
}

func TestLoadRejectsUnknownDriver(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")

	_, err := config.Load()
	require.Error(t, err)
}
