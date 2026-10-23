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
	if err := setupMongo(ctx, db); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return &mongoStore{
		client:     client,
		todos:      db.Collection("todos"),
		activities: db.Collection("activities"),
	}, nil
}

// setupMongo creates the collections up front because creating a collection
// inside a multi-document transaction is not supported on every Mongo version.
func setupMongo(ctx context.Context, db *mongo.Database) error {
	existing, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	for _, name := range []string{"todos", "activities"} {
		if slices.Contains(existing, name) {
			continue
		}
		if err := db.CreateCollection(ctx, name); err != nil {
			return fmt.Errorf("create collection %s: %w", name, err)
		}
	}
	index := mongo.IndexModel{Keys: bson.D{{Key: "todo_id", Value: 1}}}
	if _, err := db.Collection("activities").Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("create activities index: %w", err)
	}
	return nil
}

// inTx is where every Mongo transaction starts. fn must use the ctx it is
// given: that ctx carries the session, which is what puts a collection call
// inside the transaction. WithTransaction may retry fn on transient errors.
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

func (s *mongoStore) Create(ctx context.Context, title string) (Todo, error) {
	todo, err := newTodo(title)
	if err != nil {
		return Todo{}, err
	}
	created := newActivity(todo.ID, ActionCreated, todo.CreatedAt)

	err = s.inTx(ctx, func(ctx context.Context) error {
		if _, err := s.todos.InsertOne(ctx, todo); err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		if _, err := s.activities.InsertOne(ctx, created); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return todo, nil
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

func (s *mongoStore) Get(ctx context.Context, id string) (Todo, error) {
	var todo Todo
	if err := s.todos.FindOne(ctx, bson.M{"_id": id}).Decode(&todo); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Todo{}, ErrNotFound
		}
		return Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return todo, nil
}

func (s *mongoStore) Update(ctx context.Context, id string, p todoPatch) (Todo, error) {
	p, err := validatePatch(p)
	if err != nil {
		return Todo{}, err
	}

	var todo Todo
	err = s.inTx(ctx, func(ctx context.Context) error {
		// A concurrent write to the same todo makes Mongo abort this
		// transaction with a write conflict; WithTransaction then retries fn.
		err := s.todos.FindOne(ctx, bson.M{"_id": id}).Decode(&todo)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get todo: %w", err)
		}

		activities := applyPatch(&todo, p, now())
		if len(activities) == 0 {
			return nil
		}
		if _, err := s.todos.ReplaceOne(ctx, bson.M{"_id": id}, todo); err != nil {
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
	return todo, nil
}

func (s *mongoStore) Delete(ctx context.Context, id string) error {
	deleted := newActivity(id, ActionDeleted, now())

	return s.inTx(ctx, func(ctx context.Context) error {
		res, err := s.todos.DeleteOne(ctx, bson.M{"_id": id})
		if err != nil {
			return fmt.Errorf("delete todo: %w", err)
		}
		if res.DeletedCount == 0 {
			return ErrNotFound
		}
		if _, err := s.activities.InsertOne(ctx, deleted); err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *mongoStore) Activities(ctx context.Context, todoID string) ([]Activity, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := s.activities.Find(ctx, bson.M{"todo_id": todoID}, opts)
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
	if err := s.client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping mongo: %w", err)
	}
	return nil
}
