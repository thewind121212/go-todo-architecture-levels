// Command api serves the todo HTTP API. main is the composition root: it
// reads config, opens the database and wires the features together by hand.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level04/internal/activity"
	"example.com/todo/level04/internal/platform/database"
	"example.com/todo/level04/internal/platform/httpx"
	"example.com/todo/level04/internal/platform/tx"
	"example.com/todo/level04/internal/todo"
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
		postgresDSN: getenv("POSTGRES_DSN", "postgres://todo:todo@localhost:5432/todo_l04?sslmode=disable"),
		mongoURI:    getenv("MONGO_URI", "mongodb://localhost:27017/?replicaSet=rs0&directConnection=true"),
		mongoDB:     getenv("MONGO_DB", "todo_l04"),
		httpAddr:    getenv("HTTP_ADDR", ":8084"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// storage is everything that depends on the chosen database.
type storage struct {
	tx         todo.TxManager
	todos      todo.Repository
	activities activity.Repository
	ping       func(context.Context) error
	close      func(context.Context) error
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if err := run(log); err != nil {
		log.Fatal().Err(err).Msg("server failed")
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

	// Cross-feature wiring: *activity.Service satisfies todo.ActivityRecorder,
	// so main is the only place that knows both features.
	activities := activity.NewService(store.activities)
	todos := todo.NewService(store.todos, store.tx, activities)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", health(store.ping, log))
	todo.NewHandler(todos, log).Register(router)
	activity.NewHandler(activities, log).Register(router)

	srv := &http.Server{Addr: cfg.httpAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info().Str("addr", cfg.httpAddr).Str("driver", cfg.driver).Msg("listening")

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func openStorage(ctx context.Context, cfg config) (storage, error) {
	switch cfg.driver {
	case "postgres":
		return openPostgres(ctx, cfg.postgresDSN)
	case "mongo":
		return openMongo(ctx, cfg.mongoURI, cfg.mongoDB)
	default:
		return storage{}, fmt.Errorf("unknown DB_DRIVER %q: want postgres or mongo", cfg.driver)
	}
}

func openPostgres(ctx context.Context, dsn string) (storage, error) {
	db, err := database.OpenPostgres(ctx, dsn)
	if err != nil {
		return storage{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return storage{}, fmt.Errorf("postgres pool: %w", err)
	}
	// Each feature migrates its own table.
	if err := todo.MigratePostgres(ctx, db); err != nil {
		return storage{}, err
	}
	if err := activity.MigratePostgres(ctx, db); err != nil {
		return storage{}, err
	}
	return storage{
		tx:         tx.NewPostgresManager(db),
		todos:      todo.NewPostgresRepository(db),
		activities: activity.NewPostgresRepository(db),
		ping:       sqlDB.PingContext,
		close:      func(context.Context) error { return sqlDB.Close() },
	}, nil
}

func openMongo(ctx context.Context, uri, dbName string) (storage, error) {
	client, err := database.OpenMongo(ctx, uri)
	if err != nil {
		return storage{}, err
	}
	db := client.Database(dbName)
	// Each feature creates its own collection and indexes.
	if err := todo.MigrateMongo(ctx, db); err != nil {
		return storage{}, err
	}
	if err := activity.MigrateMongo(ctx, db); err != nil {
		return storage{}, err
	}
	return storage{
		tx:         tx.NewMongoManager(client),
		todos:      todo.NewMongoRepository(db),
		activities: activity.NewMongoRepository(db),
		ping:       func(ctx context.Context) error { return client.Ping(ctx, nil) },
		close:      client.Disconnect,
	}, nil
}

func health(ping func(context.Context) error, log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ping(c.Request.Context()); err != nil {
			httpx.InternalError(c, log, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
