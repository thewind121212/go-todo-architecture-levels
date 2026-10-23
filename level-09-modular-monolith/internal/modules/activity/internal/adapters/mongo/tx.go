package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// TxManager implements application.TxManager with a MongoDB session transaction: the
// activity module's own, opened for each event it records.
type TxManager struct {
	client *mongo.Client
}

var _ application.TxManager = (*TxManager)(nil)

func NewTxManager(db *mongo.Database) *TxManager {
	return &TxManager{client: db.Client()}
}

// WithinTx runs fn in a multi-document transaction on a session of its own. The ctx that
// WithTransaction passes to its callback carries that session, and the adapters use that
// ctx for every call, so all of them join the transaction. An error from fn aborts it.
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
