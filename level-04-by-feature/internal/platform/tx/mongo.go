package tx

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoManager starts MongoDB transactions.
type MongoManager struct {
	client *mongo.Client
}

func NewMongoManager(client *mongo.Client) *MongoManager {
	return &MongoManager{client: client}
}

// WithinTx is where a transaction starts. Unlike Postgres, there is no Conn
// helper: driver v2 passes fn a ctx that already carries the session, and
// every collection call made with that ctx joins the transaction.
//
// WithTransaction commits when fn returns nil, aborts when it returns an
// error, and may re-run fn on transient errors, so fn must not have side
// effects outside the database.
func (m *MongoManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := m.client.StartSession()
	if err != nil {
		return fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}
