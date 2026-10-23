// Package config reads the app settings from environment variables.
package config

import "os"

// Supported values of DB_DRIVER.
const (
	DriverPostgres = "postgres"
	DriverMongo    = "mongo"
)

// Config holds every setting the app reads at startup.
type Config struct {
	DBDriver    string
	PostgresDSN string
	MongoURI    string
	MongoDB     string
	HTTPAddr    string
}

// Load reads the environment, falling back to this level's defaults.
func Load() Config {
	return Config{
		DBDriver:    getenv("DB_DRIVER", DriverPostgres),
		PostgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l06?sslmode=disable"),
		MongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		MongoDB:     getenv("MONGO_DB", "todo_l06"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8086"),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
