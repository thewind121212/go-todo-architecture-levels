package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level03/internal/model"
)

type activityDoc struct {
	ID        string    `bson:"_id"`
	TodoID    string    `bson:"todo_id"`
	Action    string    `bson:"action"`
	CreatedAt time.Time `bson:"created_at"`
}

type ActivityRepository struct {
	coll *mongo.Collection
}

func NewActivityRepository(db *mongo.Database) *ActivityRepository {
	return &ActivityRepository{coll: db.Collection(activitiesCollection)}
}

func (r *ActivityRepository) Create(ctx context.Context, a model.Activity) error {
	doc := activityDoc{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodo(ctx context.Context, todoID string) ([]model.Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	var docs []activityDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	activities := make([]model.Activity, 0, len(docs))
	for _, doc := range docs {
		activities = append(activities, model.Activity{
			ID: doc.ID, TodoID: doc.TodoID, Action: model.Action(doc.Action), CreatedAt: doc.CreatedAt.UTC(),
		})
	}
	return activities, nil
}
