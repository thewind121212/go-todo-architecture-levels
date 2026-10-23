package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// ActivityRepository implements ports.ActivityRepository.
type ActivityRepository struct {
	coll *mongo.Collection
}

var _ ports.ActivityRepository = (*ActivityRepository)(nil)

func NewActivityRepository(db *mongo.Database) *ActivityRepository {
	return &ActivityRepository{coll: db.Collection(activitiesCollection)}
}

func (r *ActivityRepository) Append(ctx context.Context, activity *domain.Activity) error {
	if _, err := r.coll.InsertOne(ctx, toActivityDoc(activity)); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodo(ctx context.Context, todoID string) ([]*domain.Activity, error) {
	// UUIDv7 strings sort in creation order, so _id ascending means chronological.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := r.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
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
