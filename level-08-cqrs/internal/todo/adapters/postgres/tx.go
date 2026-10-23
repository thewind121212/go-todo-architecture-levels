package postgres

import (
	"context"

	"gorm.io/gorm"

	"example.com/todo/level08/internal/todo/outbox"
)

type txKey struct{}

// TxManager implements outbox.TxManager with a GORM transaction. Only the relay uses it:
// its transaction spans two stores (a read model and the outbox), so no single adapter
// method can own it the way TodoRepository.Save owns the write transaction.
type TxManager struct {
	db *gorm.DB
}

var _ outbox.TxManager = (*TxManager)(nil)

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTx starts the transaction and hands it to the adapters through ctx.
// GORM commits when fn returns nil and rolls back when it returns an error.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// conn returns the transaction stored in ctx by WithinTx, or the plain pool outside one.
// The views and the outbox use it, so the same code serves queries and the relay.
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
