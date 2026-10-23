package tx

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoManager starts MongoDB multi-document transactions.
type MongoManager struct {
	client *mongo.Client
}

func NewMongoManager(client *mongo.Client) *MongoManager {
	return &MongoManager{client: client}
}

// WithinTx commits when fn returns nil and aborts otherwise. Unlike GORM,
// Mongo needs no PostgresDB-style helper: the ctx handed to fn already
// carries the session, and every collection call made with it joins the
// transaction. The driver may retry fn on transient errors.
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
