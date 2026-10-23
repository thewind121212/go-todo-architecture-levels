package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
)

// ActivityLog implements ports.ActivityLog. The documents are written by TodoRepository.Save.
type ActivityLog struct {
	coll *mongo.Collection
}

var _ ports.ActivityLog = (*ActivityLog)(nil)

func NewActivityLog(db *mongo.Database) *ActivityLog {
	return &ActivityLog{coll: db.Collection(activitiesCollection)}
}

func (l *ActivityLog) ListByTodo(ctx context.Context, todoID domain.TodoID) ([]*domain.Activity, error) {
	// UUIDv7 strings sort in creation order, so _id ascending means chronological.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := l.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID.String()}}, opts)
	if err != nil {
		return nil, fmt.Errorf("find activities: %w", err)
	}
	var docs []activityDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	activities := make([]*domain.Activity, 0, len(docs))
	for i := range docs {
		activities = append(activities, toDomainActivity(&docs[i]))
	}
	return activities, nil
}
