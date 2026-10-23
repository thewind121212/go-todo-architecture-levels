// Level 2: the level 1 app split into one file per concern. It is still one
// package main, so the files are only a convention: any file can use any other.
//
// main.go: configuration and wiring.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	s, err := openStore(ctx, getenv("DB_DRIVER", "postgres"))
	if err != nil {
		log.Fatal(err)
	}
	if err := newRouter(s).Run(getenv("HTTP_ADDR", ":8082")); err != nil {
		log.Fatal(err)
	}
}

// openStore is the manual dependency injection: config picks the store,
// and everything else only sees the store interface.
func openStore(ctx context.Context, driver string) (store, error) {
	switch driver {
	case "postgres":
		dsn := getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l02?sslmode=disable")
		s, err := newPostgresStore(ctx, dsn)
		if err != nil {
			// Return a literal nil: a nil *postgresStore wrapped in the store
			// interface would not compare equal to nil.
			return nil, err
		}
		return s, nil
	case "mongo":
		uri := getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true")
		s, err := newMongoStore(ctx, uri, getenv("MONGO_DB", "todo_l02"))
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown DB_DRIVER %q: want postgres or mongo", driver)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
