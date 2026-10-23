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

	"example.com/todo/level07/internal/config"
	"example.com/todo/level07/internal/infrastructure/database"
	"example.com/todo/level07/internal/infrastructure/health"
	todohttp "example.com/todo/level07/internal/todo/adapters/http"
	mongoadapter "example.com/todo/level07/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level07/internal/todo/adapters/postgres"
	"example.com/todo/level07/internal/todo/application"
	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
)

// appSet is everything that does not depend on the database choice:
// the application core and the driving HTTP adapter.
var appSet = wire.NewSet(
	application.NewTodoService,
	wire.Bind(new(ports.TodoService), new(*application.TodoService)),
	ProvideRouter,
)

// PostgresSet plugs the Postgres adapters into the repository and the driven port.
// Each wire.Bind reads as "this port is implemented by that adapter".
var PostgresSet = wire.NewSet(
	appSet,
	ProvidePostgres,
	ProvidePostgresCheck,
	pgadapter.NewTodoRepository,
	pgadapter.NewActivityLog,
	wire.Bind(new(domain.TodoRepository), new(*pgadapter.TodoRepository)),
	wire.Bind(new(ports.ActivityLog), new(*pgadapter.ActivityLog)),
)

// MongoSet plugs the Mongo adapters into the same interfaces.
var MongoSet = wire.NewSet(
	appSet,
	ProvideMongo,
	ProvideMongoCheck,
	mongoadapter.NewTodoRepository,
	mongoadapter.NewActivityLog,
	wire.Bind(new(domain.TodoRepository), new(*mongoadapter.TodoRepository)),
	wire.Bind(new(ports.ActivityLog), new(*mongoadapter.ActivityLog)),
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
