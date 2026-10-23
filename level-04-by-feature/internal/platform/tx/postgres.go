// Package tx runs a unit of work in one database transaction that every
// feature's repositories can join through the context.
package tx

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// PostgresManager starts GORM transactions.
type PostgresManager struct {
	db *gorm.DB
}

func NewPostgresManager(db *gorm.DB) *PostgresManager {
	return &PostgresManager{db: db}
}

// WithinTx is where a transaction starts. GORM commits when fn returns nil
// and rolls back when it returns an error (or panics). The *gorm.DB bound to
// the transaction travels inside ctx, so the todo and activity repositories
// both write through it without knowing about each other.
func (m *PostgresManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Conn returns the transaction stored in ctx by WithinTx, or the plain
// connection when the call is not part of a transaction (e.g. reads).
// Postgres repositories in every feature start their queries here.
func Conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
