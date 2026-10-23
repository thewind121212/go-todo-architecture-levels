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

	"example.com/todo/level08/internal/config"
	"example.com/todo/level08/internal/infrastructure/database"
	"example.com/todo/level08/internal/infrastructure/health"
	todohttp "example.com/todo/level08/internal/todo/adapters/http"
	mongoadapter "example.com/todo/level08/internal/todo/adapters/mongo"
	pgadapter "example.com/todo/level08/internal/todo/adapters/postgres"
	"example.com/todo/level08/internal/todo/application/command"
	"example.com/todo/level08/internal/todo/application/projection"
	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
	"example.com/todo/level08/internal/todo/ports"
)

// App is what main runs: the HTTP router, and the relay that keeps the read models up to date.
type App struct {
	Router *gin.Engine
	Relay  *outbox.Relay
}

// appSet is everything that does not depend on the database choice: both sides of
// the application, the projector and relay between them, and the driving HTTP adapter.
var appSet = wire.NewSet(
	command.NewService,
	wire.Bind(new(ports.TodoCommands), new(*command.Service)),
	query.NewService,
	wire.Bind(new(ports.TodoQueries), new(*query.Service)),
	projection.NewProjector,
	wire.Bind(new(outbox.Handler), new(*projection.Projector)),
	ProvideRelay,
	ProvideRouter,
	wire.Struct(new(App), "*"),
)

// PostgresSet plugs the Postgres adapters into the repository and the driven ports.
// Each wire.Bind reads as "this port is implemented by that adapter".
var PostgresSet = wire.NewSet(
	appSet,
	ProvidePostgres,
	ProvidePostgresCheck,
	pgadapter.NewTodoRepository,
	pgadapter.NewTodoViews,
	pgadapter.NewActivityViews,
	pgadapter.NewOutbox,
	pgadapter.NewTxManager,
	wire.Bind(new(domain.TodoRepository), new(*pgadapter.TodoRepository)),
	wire.Bind(new(query.TodoViews), new(*pgadapter.TodoViews)),
	wire.Bind(new(query.ActivityViews), new(*pgadapter.ActivityViews)),
	wire.Bind(new(outbox.Store), new(*pgadapter.Outbox)),
	wire.Bind(new(outbox.TxManager), new(*pgadapter.TxManager)),
)

// MongoSet plugs the Mongo adapters into the same interfaces.
var MongoSet = wire.NewSet(
	appSet,
	ProvideMongo,
	ProvideMongoCheck,
	mongoadapter.NewTodoRepository,
	mongoadapter.NewTodoViews,
	mongoadapter.NewActivityViews,
	mongoadapter.NewOutbox,
	mongoadapter.NewTxManager,
	wire.Bind(new(domain.TodoRepository), new(*mongoadapter.TodoRepository)),
	wire.Bind(new(query.TodoViews), new(*mongoadapter.TodoViews)),
	wire.Bind(new(query.ActivityViews), new(*mongoadapter.ActivityViews)),
	wire.Bind(new(outbox.Store), new(*mongoadapter.Outbox)),
	wire.Bind(new(outbox.TxManager), new(*mongoadapter.TxManager)),
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

// ProvideRelay builds the relay with the poll interval from the config.
func ProvideRelay(cfg config.Config, store outbox.Store, tx outbox.TxManager, handler outbox.Handler, log zerolog.Logger) *outbox.Relay {
	return outbox.NewRelay(store, tx, handler, cfg.OutboxPollInterval, log)
}

// ProvideRouter mounts every route. The HTTP adapter only sees the two driving ports.
func ProvideRouter(commands ports.TodoCommands, queries ports.TodoQueries, check health.CheckFunc, log zerolog.Logger) *gin.Engine {
	// gin.New instead of gin.Default: requestLogger replaces Gin's text
	// logger, so each request is logged once, as zerolog JSON.
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(log))
	health.RegisterRoutes(router, check, log)
	todohttp.RegisterRoutes(router, commands, queries, log)
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
