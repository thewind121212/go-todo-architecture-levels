// Package config reads the app's settings from environment variables.
package config

import (
	"fmt"
	"os"
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
}

// Load reads the environment and fails fast on an unknown DB_DRIVER.
func Load() (Config, error) {
	cfg := Config{
		DBDriver:    getenv("DB_DRIVER", DriverPostgres),
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l07?sslmode=disable"),
		MongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		MongoDB:     getenv("MONGO_DB", "todo_l07"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8087"),
	}
	if cfg.DBDriver != DriverPostgres && cfg.DBDriver != DriverMongo {
		return Config{}, fmt.Errorf("unknown DB_DRIVER %q: use %q or %q", cfg.DBDriver, DriverPostgres, DriverMongo)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
