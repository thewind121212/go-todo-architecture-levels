// Package tx runs a unit of work in one database transaction that every
// feature's repository can join through the context.
package tx

import (
	"context"

	"gorm.io/gorm"
)

type gormTxKey struct{}

// PostgresManager starts GORM transactions.
type PostgresManager struct {
	db *gorm.DB
}

func NewPostgresManager(db *gorm.DB) *PostgresManager {
	return &PostgresManager{db: db}
}

// WithinTx commits when fn returns nil and rolls back otherwise. The
// transaction travels inside ctx, so the todo and activity repositories
// both write through it without knowing about each other.
func (m *PostgresManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(gtx *gorm.DB) error {
		return fn(context.WithValue(ctx, gormTxKey{}, gtx))
	})
}

// PostgresDB returns the transaction stored in ctx by WithinTx, or db when
// there is none. Postgres repositories call it for every query.
func PostgresDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if gtx, ok := ctx.Value(gormTxKey{}).(*gorm.DB); ok {
		return gtx
	}
	return db.WithContext(ctx)
}
