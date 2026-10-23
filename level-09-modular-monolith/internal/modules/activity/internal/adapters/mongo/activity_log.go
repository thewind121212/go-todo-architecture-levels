// Package mongo is a driven adapter: it implements the activity module's ActivityLog and
// TxManager with the official MongoDB driver.
package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// ActivityLog implements application.ActivityLog.
type ActivityLog struct {
	coll *mongo.Collection
}

var _ application.ActivityLog = (*ActivityLog)(nil)

func NewActivityLog(db *mongo.Database) *ActivityLog {
	return &ActivityLog{coll: db.Collection(activitiesCollection)}
}

func (l *ActivityLog) ListByTodo(ctx context.Context, todoID string) ([]*application.Activity, error) {
	// UUIDv7 strings sort in creation order, so _id ascending means chronological.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := l.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("find activities: %w", err)
	}
	var docs []activityDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	activities := make([]*application.Activity, 0, len(docs))
	for i := range docs {
		activities = append(activities, toActivity(&docs[i]))
	}
	return activities, nil
}

// Add inserts the activity only if its id (the event id) is new: an upsert whose update is
// $setOnInsert writes nothing when the document exists. A plain InsertOne would fail with a
// duplicate key instead, and inside a transaction that error aborts the whole transaction.
func (l *ActivityLog) Add(ctx context.Context, activity *application.Activity) error {
	insert := bson.D{{Key: "$setOnInsert", Value: toActivityDoc(activity)}}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := l.coll.UpdateByID(ctx, activity.ID, insert, opts); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}
