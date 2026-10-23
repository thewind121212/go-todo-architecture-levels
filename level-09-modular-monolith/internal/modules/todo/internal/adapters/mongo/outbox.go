package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level09/internal/platform/outbox"
)

// Outbox implements outbox.Store. The documents are written by TodoRepository.Save.
type Outbox struct {
	coll *mongo.Collection
}

var _ outbox.Store = (*Outbox)(nil)

func NewOutbox(db *mongo.Database) *Outbox {
	return &Outbox{coll: db.Collection(outboxCollection)}
}

func (o *Outbox) Unprocessed(ctx context.Context, limit int) ([]outbox.Message, error) {
	// UUIDv7 strings sort in creation order, so _id ascending means oldest first.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit))
	cur, err := o.coll.Find(ctx, bson.D{{Key: "processed_at", Value: nil}}, opts)
	if err != nil {
		return nil, fmt.Errorf("find outbox: %w", err)
	}
	var docs []outboxDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode outbox: %w", err)
	}
	messages := make([]outbox.Message, 0, len(docs))
	for i := range docs {
		messages = append(messages, toMessage(&docs[i]))
	}
	return messages, nil
}

func (o *Outbox) MarkProcessed(ctx context.Context, id string, at time.Time) error {
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "processed_at", Value: at}}}}
	if _, err := o.coll.UpdateByID(ctx, id, update); err != nil {
		return fmt.Errorf("mark outbox processed: %w", err)
	}
	return nil
}
