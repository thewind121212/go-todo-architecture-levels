package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"example.com/todo/level09/internal/modules/todo/internal/application/projection"
)

// TxManager implements projection.TxManager with a MongoDB session transaction. Only the
// projector uses it: an update reads the todo view and writes it back, and the two must
// commit together. The write side does not need it: TodoRepository.Save owns its transaction.
type TxManager struct {
	client *mongo.Client
}

var _ projection.TxManager = (*TxManager)(nil)

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
