// Package database opens the database connections and prepares their schema.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Migrations are one module's SQL migrations. Each module has its own history table, so
// modules migrate independently: their version numbers never mix, and one module's schema
// can change without touching another's.
type Migrations struct {
	Table string // golang-migrate's history table in schema public, e.g. "todo_schema_migrations"
	Files fs.FS  // the module's embedded *.sql files
}

// OpenPostgres applies each module's migrations, then opens the GORM connection pool the
// modules share. The returned func closes the pool.
func OpenPostgres(ctx context.Context, dsn string, modules ...Migrations) (*gorm.DB, func(), error) {
	for _, m := range modules {
		if err := migratePostgres(dsn, m); err != nil {
			return nil, nil, err
		}
	}
	// GORM's own logger is off: errors are returned, and the HTTP adapter logs them.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("get sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, func() { _ = sqlDB.Close() }, nil
}

// PingPostgres checks that the database answers.
func PingPostgres(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// migratePostgres runs every pending migration of one module.
// It uses its own short-lived connection because closing migrate also
// closes the *sql.DB it was given.
func migratePostgres(dsn string, module Migrations) error {
	source, err := iofs.New(module.Files, ".")
	if err != nil {
		return fmt.Errorf("migration source %s: %w", module.Table, err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	// The history tables live in public, named explicitly. golang-migrate's default is
	// CURRENT_SCHEMA(), which follows search_path ("$user", public): for a user called
	// "todo" that becomes the todo module's schema as soon as it exists.
	config := &migratepgx.Config{SchemaName: "public", MigrationsTable: module.Table}
	driver, err := migratepgx.WithInstance(sqlDB, config)
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		_ = driver.Close()
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up %s: %w", module.Table, err)
	}
	return nil
}
