// Command api serves the Todo HTTP API. main is the composition root: it reads
// config, opens the chosen database and wires every layer by hand.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"example.com/todo/level03/internal/handler"
	"example.com/todo/level03/internal/repository"
	mongorepo "example.com/todo/level03/internal/repository/mongo"
	pgrepo "example.com/todo/level03/internal/repository/postgres"
	"example.com/todo/level03/internal/service"
)

type config struct {
	driver      string
	postgresDSN string
	mongoURI    string
	mongoDB     string
	httpAddr    string
}

func loadConfig() config {
	return config{
		driver:      getenv("DB_DRIVER", "postgres"),
		postgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l03?sslmode=disable"),
		mongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		mongoDB:     getenv("MONGO_DB", "todo_l03"),
		httpAddr:    getenv("HTTP_ADDR", ":8083"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// storage is everything the service and the health check need from one database.
type storage struct {
	tx         repository.TxManager
	todos      repository.TodoRepository
	activities repository.ActivityRepository
	ping       func(context.Context) error
	close      func(context.Context) error
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if err := run(log); err != nil {
		log.Fatal().Err(err).Msg("api stopped")
	}
}

func run(log zerolog.Logger) error {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := openStorage(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.close(context.Background()); err != nil {
			log.Error().Err(err).Msg("close database")
		}
	}()

	todos := service.NewTodoService(store.tx, store.todos, store.activities)
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           handler.NewRouter(todos, store.ping, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info().Str("addr", cfg.httpAddr).Str("driver", cfg.driver).Msg("listening")

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// Graceful shutdown: stop accepting connections and let in-flight requests
	// finish (bounded by the timeout) before the database is closed.
	log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func openStorage(ctx context.Context, cfg config) (storage, error) {
	switch cfg.driver {
	case "postgres":
		return openPostgres(cfg.postgresDSN)
	case "mongo":
		return openMongo(ctx, cfg.mongoURI, cfg.mongoDB)
	default:
		return storage{}, fmt.Errorf("unknown DB_DRIVER %q: want postgres or mongo", cfg.driver)
	}
}

func openPostgres(dsn string) (storage, error) {
	db, err := pgrepo.Open(dsn)
	if err != nil {
		return storage{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return storage{}, fmt.Errorf("postgres pool: %w", err)
	}
	if err := pgrepo.Migrate(db); err != nil {
		sqlDB.Close()
		return storage{}, err
	}
	return storage{
		tx:         pgrepo.NewTxManager(db),
		todos:      pgrepo.NewTodoRepository(db),
		activities: pgrepo.NewActivityRepository(db),
		ping:       sqlDB.PingContext,
		close:      func(context.Context) error { return sqlDB.Close() },
	}, nil
}

func openMongo(ctx context.Context, uri, dbName string) (storage, error) {
	client, err := mongorepo.Open(uri)
	if err != nil {
		return storage{}, err
	}
	db := client.Database(dbName)
	if err := mongorepo.Migrate(ctx, db); err != nil {
		client.Disconnect(ctx)
		return storage{}, err
	}
	return storage{
		tx:         mongorepo.NewTxManager(client),
		todos:      mongorepo.NewTodoRepository(db),
		activities: mongorepo.NewActivityRepository(db),
		ping:       func(ctx context.Context) error { return client.Ping(ctx, nil) },
		close:      client.Disconnect,
	}, nil
}
