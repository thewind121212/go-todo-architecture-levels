// Package database opens the connections that the features share.
package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenPostgres connects to Postgres. GORM's own logger is silenced:
// repositories return errors and the handlers log the ones that become a 500.
func OpenPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return db, nil
}
