package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level05/internal/entity"
)

// ActivityRepository implements usecase.ActivityRepository.
type ActivityRepository struct {
	activities *mongo.Collection
}

func NewActivityRepository(db *mongo.Database) *ActivityRepository {
	return &ActivityRepository{activities: db.Collection(activitiesCollection)}
}

func (r *ActivityRepository) Create(ctx context.Context, activity entity.Activity) error {
	if _, err := r.activities.InsertOne(ctx, toActivityDocument(activity)); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodo(ctx context.Context, todoID string) ([]entity.Activity, error) {
	// UUIDv7 strings sort by time, so _id ascending is chronological.
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := r.activities.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	var docs []activityDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	activities := make([]entity.Activity, 0, len(docs))
	for _, d := range docs {
		activities = append(activities, toActivityEntity(d))
	}
	return activities, nil
}
