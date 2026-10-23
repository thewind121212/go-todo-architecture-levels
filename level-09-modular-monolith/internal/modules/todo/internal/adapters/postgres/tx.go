package postgres

import (
	"context"

	"gorm.io/gorm"

	"example.com/todo/level09/internal/modules/todo/internal/application/projection"
)

// txKey is unexported, so only this module's adapters can find its transaction in a ctx.
// Another module's adapters have their own key: they cannot join this transaction even
// by accident.
type txKey struct{}

// TxManager implements projection.TxManager with a GORM transaction. Only the projector
// uses it: an update reads the todo view and writes it back, and the two must commit
// together. The write side does not need it: TodoRepository.Save owns its transaction.
type TxManager struct {
	db *gorm.DB
}

var _ projection.TxManager = (*TxManager)(nil)

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
// The views and the outbox use it, so the same code serves queries, the projector and the relay.
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
