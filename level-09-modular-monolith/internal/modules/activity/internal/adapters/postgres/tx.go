package postgres

import (
	"context"

	"gorm.io/gorm"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// txKey is unexported, so only this module's adapters can find its transaction in a ctx.
type txKey struct{}

// TxManager implements application.TxManager with a GORM transaction: the activity
// module's own, opened for each event it records.
type TxManager struct {
	db *gorm.DB
}

var _ application.TxManager = (*TxManager)(nil)

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
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
