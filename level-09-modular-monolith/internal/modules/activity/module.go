// Package activity is the activity module's public surface: it keeps the audit log of every
// todo and serves GET /todos/:id/activities. It learns about todos only from the todo
// module's published events (package todo/contracts), and it owns its data: the
// activity.activities table, or collection.
package activity

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	activityhttp "example.com/todo/level09/internal/modules/activity/internal/adapters/http"
	mongoadapter "example.com/todo/level09/internal/modules/activity/internal/adapters/mongo"
	pgadapter "example.com/todo/level09/internal/modules/activity/internal/adapters/postgres"
	"example.com/todo/level09/internal/modules/activity/internal/application"
	"example.com/todo/level09/internal/modules/activity/internal/migrations"
	"example.com/todo/level09/internal/platform/database"
	"example.com/todo/level09/internal/platform/eventbus"
)

// Module is a built activity module.
type Module struct {
	service *application.Service
	log     zerolog.Logger
}

// NewPostgres builds the module on Postgres and subscribes it to the todo events on bus.
func NewPostgres(db *gorm.DB, bus *eventbus.Bus, log zerolog.Logger) *Module {
	return newModule(pgadapter.NewActivityLog(db), pgadapter.NewTxManager(db), bus, log)
}

// NewMongo builds the module on MongoDB and subscribes it to the todo events on bus.
func NewMongo(db *mongo.Database, bus *eventbus.Bus, log zerolog.Logger) *Module {
	return newModule(mongoadapter.NewActivityLog(db), mongoadapter.NewTxManager(db), bus, log)
}

func newModule(activities application.ActivityLog, tx application.TxManager, bus *eventbus.Bus, log zerolog.Logger) *Module {
	service := application.NewService(activities, tx)
	service.Subscribe(bus)
	return &Module{service: service, log: log}
}

// RegisterRoutes mounts /todos/:id/activities.
func (m *Module) RegisterRoutes(router gin.IRouter) {
	activityhttp.RegisterRoutes(router, m.service, m.log)
}

// PostgresMigrations is the module's schema: its SQL files, and its own history table.
func PostgresMigrations() database.Migrations {
	return database.Migrations{Table: "activity_schema_migrations", Files: migrations.FS}
}

// PrepareMongo creates the module's collection and index.
func PrepareMongo(ctx context.Context, db *mongo.Database) error {
	return mongoadapter.Prepare(ctx, db)
}
