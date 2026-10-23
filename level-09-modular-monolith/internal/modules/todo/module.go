// Package todo is the todo module's public surface. Everything else of the module is under
// internal/, which Go lets only this module import. So the rest of the app can do with the
// module exactly what this file offers: build it, mount its routes, run its relay. Other
// modules learn about todos only from the events in package contracts.
package todo

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	todohttp "example.com/todo/level09/internal/modules/todo/internal/adapters/http"
	mongoadapter "example.com/todo/level09/internal/modules/todo/internal/adapters/mongo"
	pgadapter "example.com/todo/level09/internal/modules/todo/internal/adapters/postgres"
	"example.com/todo/level09/internal/modules/todo/internal/application/command"
	"example.com/todo/level09/internal/modules/todo/internal/application/integration"
	"example.com/todo/level09/internal/modules/todo/internal/application/projection"
	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/modules/todo/internal/migrations"
	"example.com/todo/level09/internal/modules/todo/internal/ports"
	"example.com/todo/level09/internal/platform/database"
	"example.com/todo/level09/internal/platform/eventbus"
	"example.com/todo/level09/internal/platform/outbox"
)

// Module is a built todo module: level 8's CQRS inside, behind two methods.
type Module struct {
	commands  ports.TodoCommands
	queries   ports.TodoQueries
	outbox    outbox.Store
	publisher outbox.Handler
	log       zerolog.Logger
}

// NewPostgres builds the module on Postgres and subscribes its projector to its own events on bus.
func NewPostgres(db *gorm.DB, bus *eventbus.Bus, log zerolog.Logger) *Module {
	return newModule(
		pgadapter.NewTodoRepository(db),
		pgadapter.NewTodoViews(db),
		pgadapter.NewOutbox(db),
		pgadapter.NewTxManager(db),
		bus, log,
	)
}

// NewMongo builds the module on MongoDB and subscribes its projector to its own events on bus.
func NewMongo(db *mongo.Database, bus *eventbus.Bus, log zerolog.Logger) *Module {
	return newModule(
		mongoadapter.NewTodoRepository(db),
		mongoadapter.NewTodoViews(db),
		mongoadapter.NewOutbox(db),
		mongoadapter.NewTxManager(db),
		bus, log,
	)
}

// newModule is the module's own composition root, the same for both databases.
// Wire cannot do this part: it writes the wiring into cmd/api, which may not import internal/.
func newModule(todos domain.TodoRepository, views query.TodoViews, store outbox.Store, tx projection.TxManager, bus *eventbus.Bus, log zerolog.Logger) *Module {
	projection.NewProjector(views, tx).Subscribe(bus)
	return &Module{
		commands:  command.NewService(todos),
		queries:   query.NewService(views),
		outbox:    store,
		publisher: integration.NewPublisher(bus),
		log:       log,
	}
}

// RegisterRoutes mounts /todos and /todos/:id.
func (m *Module) RegisterRoutes(router gin.IRouter) {
	todohttp.RegisterRoutes(router, m.commands, m.queries, m.log)
}

// NewRelay returns the relay that publishes this module's outbox on the bus, polling every interval.
func (m *Module) NewRelay(interval time.Duration) *outbox.Relay {
	return outbox.NewRelay(m.outbox, m.publisher, interval, m.log)
}

// PostgresMigrations is the module's schema: its SQL files, and its own history table.
func PostgresMigrations() database.Migrations {
	return database.Migrations{Table: "todo_schema_migrations", Files: migrations.FS}
}

// PrepareMongo creates the module's collections and indexes.
func PrepareMongo(ctx context.Context, db *mongo.Database) error {
	return mongoadapter.Prepare(ctx, db)
}
