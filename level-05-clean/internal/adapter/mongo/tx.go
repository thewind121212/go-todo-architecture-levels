package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// TxManager implements usecase.TxManager with a MongoDB session transaction.
type TxManager struct {
	client *mongo.Client
}

func NewTxManager(db *mongo.Database) *TxManager {
	return &TxManager{client: db.Client()}
}

// WithinTx runs fn in a transaction. The ctx that WithTransaction passes to its callback carries the
// session, and every collection call made with that ctx joins the transaction. So, unlike the Postgres
// adapter, the repositories need no extra code. WithTransaction may retry fn on transient errors.
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
