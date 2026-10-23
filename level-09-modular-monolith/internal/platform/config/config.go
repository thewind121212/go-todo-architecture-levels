// Package config reads the app's settings from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

const (
	DriverPostgres = "postgres"
	DriverMongo    = "mongo"
)

type Config struct {
	DBDriver    string
	PostgresDSN string
	MongoURI    string
	MongoDB     string
	HTTPAddr    string
	// OutboxPollInterval is how often the relay looks for new events, so it is also how far
	// the read models may lag behind a write.
	OutboxPollInterval time.Duration
}

// Load reads the environment and fails fast on an unknown DB_DRIVER or a bad poll interval.
func Load() (Config, error) {
	cfg := Config{
		DBDriver:    getenv("DB_DRIVER", DriverPostgres),
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l09?sslmode=disable"),
		MongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		MongoDB:     getenv("MONGO_DB", "todo_l09"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8089"),
	}
	if cfg.DBDriver != DriverPostgres && cfg.DBDriver != DriverMongo {
		return Config{}, fmt.Errorf("unknown DB_DRIVER %q: use %q or %q", cfg.DBDriver, DriverPostgres, DriverMongo)
	}
	raw := getenv("OUTBOX_POLL_INTERVAL", "100ms")
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return Config{}, fmt.Errorf("invalid OUTBOX_POLL_INTERVAL %q: use a positive duration like 100ms", raw)
	}
	cfg.OutboxPollInterval = interval
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
