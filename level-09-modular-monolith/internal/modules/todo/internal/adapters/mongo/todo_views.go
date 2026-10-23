package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"example.com/todo/level09/internal/modules/todo/internal/application/query"
)

// TodoViews implements query.TodoViews. Queries read it; the projector writes it
// inside its own transaction.
type TodoViews struct {
	coll *mongo.Collection
}

var _ query.TodoViews = (*TodoViews)(nil)

func NewTodoViews(db *mongo.Database) *TodoViews {
	return &TodoViews{coll: db.Collection(todoViewsCollection)}
}

func (v *TodoViews) Get(ctx context.Context, id string) (*query.TodoView, error) {
	var d todoViewDoc
	err := v.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, query.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find todo view: %w", err)
	}
	return toTodoView(&d), nil
}

func (v *TodoViews) List(ctx context.Context, limit, offset int) ([]*query.TodoView, error) {
	// UUIDv7 strings sort in creation order, so _id descending means newest first.
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cur, err := v.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find todo views: %w", err)
	}
	var docs []todoViewDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode todo views: %w", err)
	}
	views := make([]*query.TodoView, 0, len(docs))
	for i := range docs {
		views = append(views, toTodoView(&docs[i]))
	}
	return views, nil
}

// Upsert replaces the document with the view's id, or inserts it: it ends up equal to view either way.
func (v *TodoViews) Upsert(ctx context.Context, view *query.TodoView) error {
	opts := options.Replace().SetUpsert(true)
	if _, err := v.coll.ReplaceOne(ctx, bson.D{{Key: "_id", Value: view.ID}}, toTodoViewDoc(view), opts); err != nil {
		return fmt.Errorf("upsert todo view: %w", err)
	}
	return nil
}

func (v *TodoViews) Delete(ctx context.Context, id string) error {
	if _, err := v.coll.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}}); err != nil {
		return fmt.Errorf("delete todo view: %w", err)
	}
	return nil
}
