package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

type config struct {
	dbDriver    string
	postgresDSN string
	mongoURI    string
	mongoDB     string
	httpAddr    string
}

func loadConfig() config {
	return config{
		dbDriver:    getenv("DB_DRIVER", "postgres"),
		postgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l02?sslmode=disable"),
		mongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		mongoDB:     getenv("MONGO_DB", "todo_l02"),
		httpAddr:    getenv("HTTP_ADDR", ":8082"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// openStore is the only place that knows both implementations exist.
// Each case checks err itself so a nil *postgresStore is never returned
// wrapped in a non-nil store interface.
func openStore(ctx context.Context, cfg config) (store, error) {
	switch cfg.dbDriver {
	case "postgres":
		s, err := newPostgresStore(cfg.postgresDSN)
		if err != nil {
			return nil, err
		}
		return s, nil
	case "mongo":
		s, err := newMongoStore(ctx, cfg.mongoURI, cfg.mongoDB)
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown DB_DRIVER %q (want postgres or mongo)", cfg.dbDriver)
	}
}

func main() {
	cfg := loadConfig()

	s, err := openStore(context.Background(), cfg)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	router := newRouter(s)
	log.Printf("listening on %s (DB_DRIVER=%s)", cfg.httpAddr, cfg.dbDriver)
	if err := router.Run(cfg.httpAddr); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
