package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level08/internal/todo/application/query"
)

// ActivityViews implements query.ActivityViews. Queries read it; the projector writes it
// inside the relay's transaction.
type ActivityViews struct {
	coll *mongo.Collection
}

var _ query.ActivityViews = (*ActivityViews)(nil)

func NewActivityViews(db *mongo.Database) *ActivityViews {
	return &ActivityViews{coll: db.Collection(activityViewsCollection)}
}

func (v *ActivityViews) ListByTodo(ctx context.Context, todoID string) ([]*query.ActivityView, error) {
	// UUIDv7 strings sort in creation order, so _id ascending means chronological.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := v.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("find activity views: %w", err)
	}
	var docs []activityViewDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activity views: %w", err)
	}
	activities := make([]*query.ActivityView, 0, len(docs))
	for i := range docs {
		activities = append(activities, toActivityView(&docs[i]))
	}
	return activities, nil
}

// Add inserts the activity only if its id (the event id) is new: an upsert whose update is
// $setOnInsert writes nothing when the document exists. A plain InsertOne would fail with a
// duplicate key instead, and inside a transaction that error aborts the whole transaction.
func (v *ActivityViews) Add(ctx context.Context, activity *query.ActivityView) error {
	insert := bson.D{{Key: "$setOnInsert", Value: toActivityViewDoc(activity)}}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := v.coll.UpdateByID(ctx, activity.ID, insert, opts); err != nil {
		return fmt.Errorf("insert activity view: %w", err)
	}
	return nil
}
