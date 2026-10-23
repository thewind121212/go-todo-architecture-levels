// Package wire holds the Wire provider sets: the list of constructors Wire
// may use to build the app. Each set below is one complete way to plug
// adapters into the ports.
package wire

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"example.com/todo/level06/internal/config"
	"example.com/todo/level06/internal/infrastructure/database"
	"example.com/todo/level06/internal/infrastructure/health"
	todohttp "example.com/todo/level06/internal/todo/adapters/http"
	mongoadapter "example.com/todo/level06/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level06/internal/todo/adapters/postgres"
	"example.com/todo/level06/internal/todo/ports"
	"example.com/todo/level06/internal/todo/service"
)

// appSet is everything that does not depend on the database choice:
// the application core and the driving HTTP adapter.
var appSet = wire.NewSet(
	service.NewTodoService,
	wire.Bind(new(ports.TodoService), new(*service.TodoService)),
	ProvideRouter,
)

// PostgresSet plugs the Postgres adapters into the driven ports.
// Each wire.Bind reads as "this port is implemented by that adapter".
var PostgresSet = wire.NewSet(
	appSet,
	ProvidePostgres,
	ProvidePostgresCheck,
	pgadapter.NewTodoRepository,
	pgadapter.NewActivityRepository,
	pgadapter.NewTxManager,
	wire.Bind(new(ports.TodoRepository), new(*pgadapter.TodoRepository)),
	wire.Bind(new(ports.ActivityRepository), new(*pgadapter.ActivityRepository)),
	wire.Bind(new(ports.TxManager), new(*pgadapter.TxManager)),
)

// MongoSet plugs the Mongo adapters into the same driven ports.
var MongoSet = wire.NewSet(
	appSet,
	ProvideMongo,
	ProvideMongoCheck,
	mongoadapter.NewTodoRepository,
	mongoadapter.NewActivityRepository,
	mongoadapter.NewTxManager,
	wire.Bind(new(ports.TodoRepository), new(*mongoadapter.TodoRepository)),
	wire.Bind(new(ports.ActivityRepository), new(*mongoadapter.ActivityRepository)),
	wire.Bind(new(ports.TxManager), new(*mongoadapter.TxManager)),
)

// ProvidePostgres opens the pool and migrates the schema. Wire calls the
// returned cleanup func when the app shuts down.
func ProvidePostgres(ctx context.Context, cfg config.Config) (*gorm.DB, func(), error) {
	return database.OpenPostgres(ctx, cfg.PostgresDSN)
}

func ProvidePostgresCheck(db *gorm.DB) health.CheckFunc {
	return func(ctx context.Context) error { return database.PingPostgres(ctx, db) }
}

// ProvideMongo connects and prepares collections and indexes.
func ProvideMongo(ctx context.Context, cfg config.Config) (*mongo.Database, func(), error) {
	return database.OpenMongo(ctx, cfg.MongoURI, cfg.MongoDB)
}

func ProvideMongoCheck(db *mongo.Database) health.CheckFunc {
	return func(ctx context.Context) error { return database.PingMongo(ctx, db) }
}

// ProvideRouter mounts every route. The HTTP adapter only sees ports.TodoService.
func ProvideRouter(svc ports.TodoService, check health.CheckFunc, log zerolog.Logger) *gin.Engine {
	// gin.New instead of gin.Default: requestLogger replaces Gin's text
	// logger, so each request is logged once, as zerolog JSON.
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(log))
	health.RegisterRoutes(router, check, log)
	todohttp.RegisterRoutes(router, svc, log)
	return router
}

func requestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}
