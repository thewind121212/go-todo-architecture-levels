// Package postgres implements the use case ports with GORM on PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to PostgreSQL and creates or updates the tables.
func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	// GORM's own logger is silenced: errors are returned to the caller, which logs them with zerolog.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&todoModel{}, &activityModel{}); err != nil {
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return db, nil
}
