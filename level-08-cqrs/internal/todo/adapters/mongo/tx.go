package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level08/internal/todo/outbox"
)

// TxManager implements outbox.TxManager with a MongoDB session transaction. Only the relay
// uses it: its transaction spans two stores (a read model and the outbox), so no single
// adapter method can own it the way TodoRepository.Save owns the write transaction.
type TxManager struct {
	client *mongo.Client
}

var _ outbox.TxManager = (*TxManager)(nil)

func NewTxManager(db *mongo.Database) *TxManager {
	return &TxManager{client: db.Client()}
}

// WithinTx runs fn in a multi-document transaction. The ctx that
// WithTransaction passes to its callback carries the session, and the
// adapters use that ctx for every call, so all of them join the
// transaction. An error from fn aborts it.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}
