// store_mongo.go: the Mongo store (driver v2). Each write method owns its transaction.

package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type mongoStore struct {
	client     *mongo.Client
	todos      *mongo.Collection
	activities *mongo.Collection
}

func newMongoStore(ctx context.Context, uri, dbName string) (*mongoStore, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	db := client.Database(dbName)
	// Collections are created up front because creating one inside a
	// multi-document transaction is not supported on every Mongo version.
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	for _, name := range []string{"todos", "activities"} {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return nil, fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	activities := db.Collection("activities")
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := activities.Indexes().CreateOne(ctx, index); err != nil {
		return nil, fmt.Errorf("create activities index: %w", err)
	}
	return &mongoStore{client: client, todos: db.Collection("todos"), activities: activities}, nil
}

// inTx runs fn in one multi-document transaction. fn must pass the ctx it is
// given to every collection call: that ctx carries the session, and the
// session is what puts the call inside the transaction.
func (s *mongoStore) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	session, err := s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	return err
}

func (s *mongoStore) Create(ctx context.Context, t Todo) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		if _, err := s.todos.InsertOne(ctx, t); err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		if _, err := s.activities.InsertOne(ctx, newActivity(t.ID, ActionCreated, t.CreatedAt)); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *mongoStore) Get(ctx context.Context, id string) (Todo, error) {
	var t Todo
	err := s.todos.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return t, nil
}

func (s *mongoStore) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := s.todos.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	todos := []Todo{}
	if err := cursor.All(ctx, &todos); err != nil {
		return nil, fmt.Errorf("decode todos: %w", err)
	}
	return todos, nil
}

func (s *mongoStore) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	var t Todo
	// If another transaction changes this todo first, Mongo reports a write
	// conflict and WithTransaction reruns fn, so a stale read is never committed.
	err := s.inTx(ctx, func(ctx context.Context) error {
		var err error
		t, err = s.Get(ctx, id)
		if err != nil {
			return err
		}
		activities := applyPatch(&t, p, nowUTC())
		if len(activities) == 0 {
			return nil
		}
		if _, err := s.todos.ReplaceOne(ctx, bson.D{{Key: "_id", Value: t.ID}}, t); err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if _, err := s.activities.InsertMany(ctx, activities); err != nil {
			return fmt.Errorf("insert activities: %w", err)
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return t, nil
}

func (s *mongoStore) Delete(ctx context.Context, id string) error {
	return s.inTx(ctx, func(ctx context.Context) error {
		res, err := s.todos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
		if err != nil {
			return fmt.Errorf("delete todo: %w", err)
		}
		if res.DeletedCount == 0 {
			return ErrNotFound
		}
		if _, err := s.activities.InsertOne(ctx, newActivity(id, ActionDeleted, nowUTC())); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *mongoStore) ListActivities(ctx context.Context, todoID string) ([]Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := s.activities.Find(ctx, bson.D{{Key: "todo_id", Value: todoID}}, opts)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := []Activity{}
	if err := cursor.All(ctx, &activities); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}
	return activities, nil
}

func (s *mongoStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, nil)
}
