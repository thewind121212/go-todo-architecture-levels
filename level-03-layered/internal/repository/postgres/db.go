// Package postgres implements the repository interfaces with GORM.
package postgres

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects to Postgres. GORM's own logger is silenced: repositories
// return errors and the handler logs the ones that become a 500.
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return db, nil
}

// Migrate creates or updates the tables from the row structs.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&todoRow{}, &activityRow{}); err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	return nil
}

type txKey struct{}

// TxManager starts GORM transactions for the service.
type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTx is where a transaction starts. GORM commits when fn returns nil
// and rolls back when it returns an error (or panics). The *gorm.DB bound to
// the transaction travels to the repositories inside ctx.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// conn returns the transaction stored in ctx by WithinTx, or the plain
// connection when the call is not part of a transaction (e.g. reads).
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
