package config_test

import (
	"testing"

	"example.com/todo/level05/internal/infrastructure/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("HTTP_ADDR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBDriver != config.DriverPostgres || cfg.HTTPAddr != ":8085" || cfg.MongoDB != "todo_l05" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsUnknownDriver(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")

	if _, err := config.Load(); err == nil {
		t.Error("want an error for an unknown DB_DRIVER")
	}
}
