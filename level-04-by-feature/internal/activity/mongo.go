package activity

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level04/internal/platform/database"
)

const activitiesCollection = "activities"

type activityDoc struct {
	ID        string    `bson:"_id"`
	TodoID    string    `bson:"todo_id"`
	Action    string    `bson:"action"`
	CreatedAt time.Time `bson:"created_at"`
}

func toActivityDoc(a Activity) activityDoc {
	return activityDoc{ID: a.ID, TodoID: a.TodoID, Action: a.Action, CreatedAt: a.CreatedAt}
}

func (d activityDoc) toActivity() Activity {
	return Activity{ID: d.ID, TodoID: d.TodoID, Action: d.Action, CreatedAt: d.CreatedAt.UTC()}
}

// MigrateMongo creates this feature's collection and its todo_id index.
func MigrateMongo(ctx context.Context, db *mongo.Database) error {
	if err := database.EnsureCollection(ctx, db, activitiesCollection); err != nil {
		return err
	}
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection(activitiesCollection).Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}

// MongoRepository just uses the ctx it is given: inside WithinTx that ctx
// carries the session, so every call joins the transaction.
type MongoRepository struct {
	coll *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{coll: db.Collection(activitiesCollection)}
}

func (r *MongoRepository) Create(ctx context.Context, a Activity) error {
	if _, err := r.coll.InsertOne(ctx, toActivityDoc(a)); err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *MongoRepository) ListByTodo(ctx context.Context, todoID string) ([]Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	var docs []activityDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	activities := make([]Activity, 0, len(docs))
	for _, doc := range docs {
		activities = append(activities, doc.toActivity())
	}
	return activities, nil
}
