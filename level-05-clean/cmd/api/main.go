// Command api is the composition root: the only place that knows every concrete type. It builds the
// adapters, hands them to the use cases as ports, and hands the use cases to the controllers.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	httpadapter "example.com/todo/level05/internal/adapter/http"
	"example.com/todo/level05/internal/adapter/mongo"
	"example.com/todo/level05/internal/adapter/postgres"
	"example.com/todo/level05/internal/infrastructure/config"
	"example.com/todo/level05/internal/infrastructure/server"
	"example.com/todo/level05/internal/usecase"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if err := run(log); err != nil {
		log.Fatal().Err(err).Msg("stopped")
	}
}

func run(log zerolog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := openStorage(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.close(context.WithoutCancel(ctx)); err != nil {
			log.Error().Err(err).Msg("close storage")
		}
	}()
	log.Info().Str("driver", cfg.DBDriver).Msg("storage ready")

	todos := httpadapter.NewTodoController(
		usecase.NewCreateTodo(store.todos, store.activities, store.tx),
		usecase.NewGetTodo(store.todos),
		usecase.NewListTodos(store.todos),
		usecase.NewUpdateTodo(store.todos, store.activities, store.tx),
		usecase.NewDeleteTodo(store.todos, store.activities, store.tx),
		log,
	)
	activities := httpadapter.NewActivityController(usecase.NewListActivities(store.activities), log)

	gin.SetMode(gin.ReleaseMode)
	router := httpadapter.NewRouter(todos, activities, store.ping, log)
	return server.Run(ctx, cfg.HTTPAddr, router, log)
}

// storage is everything the rest of the app needs from the chosen database, typed as ports.
type storage struct {
	todos      usecase.TodoRepository
	activities usecase.ActivityRepository
	tx         usecase.TxManager
	ping       func(context.Context) error
	close      func(context.Context) error
}

func openStorage(ctx context.Context, cfg config.Config) (storage, error) {
	switch cfg.DBDriver {
	case config.DriverPostgres:
		db, err := postgres.Open(ctx, cfg.PostgresDSN)
		if err != nil {
			return storage{}, err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return storage{}, fmt.Errorf("postgres pool: %w", err)
		}
		return storage{
			todos:      postgres.NewTodoRepository(db),
			activities: postgres.NewActivityRepository(db),
			tx:         postgres.NewTxManager(db),
			ping:       sqlDB.PingContext,
			close:      func(context.Context) error { return sqlDB.Close() },
		}, nil
	case config.DriverMongo:
		db, err := mongo.Open(ctx, cfg.MongoURI, cfg.MongoDB)
		if err != nil {
			return storage{}, err
		}
		return storage{
			todos:      mongo.NewTodoRepository(db),
			activities: mongo.NewActivityRepository(db),
			tx:         mongo.NewTxManager(db),
			ping:       func(ctx context.Context) error { return db.Client().Ping(ctx, nil) },
			close:      db.Client().Disconnect,
		}, nil
	default:
		return storage{}, fmt.Errorf("unknown DB_DRIVER %q", cfg.DBDriver)
	}
}
