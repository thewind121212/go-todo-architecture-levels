package mongo

import (
	"time"

	"example.com/todo/level05/internal/entity"
)

// The BSON documents carry the bson tags, so the entities don't have to.

type todoDocument struct {
	ID        string    `bson:"_id"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

type activityDocument struct {
	ID        string    `bson:"_id"`
	TodoID    string    `bson:"todo_id"`
	Action    string    `bson:"action"`
	CreatedAt time.Time `bson:"created_at"`
}

func toTodoDocument(t entity.Todo) todoDocument {
	return todoDocument{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func toTodoEntity(d todoDocument) entity.Todo {
	return entity.Todo{ID: d.ID, Title: d.Title, Completed: d.Completed, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

func toActivityDocument(a entity.Activity) activityDocument {
	return activityDocument{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
}

func toActivityEntity(d activityDocument) entity.Activity {
	return entity.Activity{ID: d.ID, TodoID: d.TodoID, Action: entity.Action(d.Action), CreatedAt: d.CreatedAt}
}
