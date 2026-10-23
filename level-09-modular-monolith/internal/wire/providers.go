// Package wire holds the Wire provider sets: the list of constructors Wire may use to build
// the app. Each set below is one complete app: the platform, plus every module built on one
// storage choice. Wire sees each module only through its public constructor; how a module
// wires its own insides is the module's business.
package wire

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"example.com/todo/level09/internal/modules/activity"
	"example.com/todo/level09/internal/modules/todo"
	"example.com/todo/level09/internal/platform/config"
	"example.com/todo/level09/internal/platform/database"
	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/health"
	"example.com/todo/level09/internal/platform/outbox"
)

// App is what main runs: the HTTP router, and the relay that delivers the todo module's events.
type App struct {
	Router *gin.Engine
	Relay  *outbox.Relay
}

// appSet is everything that does not depend on the database choice: the event bus that
// connects the modules, the relay, the router, and the App itself.
var appSet = wire.NewSet(
	eventbus.New,
	ProvideRelay,
	ProvideRouter,
	wire.Struct(new(App), "*"),
)

// PostgresSet builds every module on Postgres. They share one pool; each has its own schema.
var PostgresSet = wire.NewSet(
	appSet,
	ProvidePostgres,
	ProvidePostgresCheck,
	todo.NewPostgres,
	activity.NewPostgres,
)

// MongoSet builds every module on MongoDB. They share one database; each has its own collections.
var MongoSet = wire.NewSet(
	appSet,
	ProvideMongo,
	ProvideMongoCheck,
	todo.NewMongo,
	activity.NewMongo,
)

// ProvidePostgres migrates each module's schema, then opens the pool. Wire calls the
// returned cleanup func when the app shuts down.
func ProvidePostgres(ctx context.Context, cfg config.Config) (*gorm.DB, func(), error) {
	return database.OpenPostgres(ctx, cfg.PostgresDSN, todo.PostgresMigrations(), activity.PostgresMigrations())
}

func ProvidePostgresCheck(db *gorm.DB) health.CheckFunc {
	return func(ctx context.Context) error { return database.PingPostgres(ctx, db) }
}

// ProvideMongo connects, then lets each module create its collections and indexes.
func ProvideMongo(ctx context.Context, cfg config.Config) (*mongo.Database, func(), error) {
	return database.OpenMongo(ctx, cfg.MongoURI, cfg.MongoDB, todo.PrepareMongo, activity.PrepareMongo)
}

func ProvideMongoCheck(db *mongo.Database) health.CheckFunc {
	return func(ctx context.Context) error { return database.PingMongo(ctx, db) }
}

// ProvideRelay builds the todo module's relay with the poll interval from the config.
// Only the todo module publishes events, so there is one relay.
func ProvideRelay(cfg config.Config, todos *todo.Module) *outbox.Relay {
	return todos.NewRelay(cfg.OutboxPollInterval)
}

// ProvideRouter mounts every module's routes on one router.
func ProvideRouter(todos *todo.Module, activities *activity.Module, check health.CheckFunc, log zerolog.Logger) *gin.Engine {
	// gin.New instead of gin.Default: requestLogger replaces Gin's text
	// logger, so each request is logged once, as zerolog JSON.
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(log))
	health.RegisterRoutes(router, check, log)
	todos.RegisterRoutes(router)
	activities.RegisterRoutes(router)
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
