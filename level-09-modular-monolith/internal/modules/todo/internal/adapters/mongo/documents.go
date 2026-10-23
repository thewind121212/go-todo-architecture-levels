// Package mongo is a driven adapter: it implements domain.TodoRepository (write side),
// query.TodoViews (read side), projection.TxManager (projector) and outbox.Store (relay)
// with the official MongoDB driver.
package mongo

import "time"

// Collection names. Mongo has no schemas, so the module's name is a prefix, the way
// Postgres qualifies a table with its schema: "todo.todos" in both. Prepare creates them.
const (
	todosCollection     = "todo.todos"
	outboxCollection    = "todo.outbox"
	todoViewsCollection = "todo.todo_views"
)

// todoDoc is the BSON shape of a todo, kept apart from domain.Todo.
type todoDoc struct {
	ID        string    `bson:"_id"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	Version   int       `bson:"version"`
}

// outboxDoc is the BSON shape of an outbox entry: one serialized domain event.
type outboxDoc struct {
	ID          string     `bson:"_id"`
	TodoID      string     `bson:"todo_id"`
	Name        string     `bson:"name"`
	Payload     string     `bson:"payload"` // the JSON text, as the codec wrote it
	OccurredAt  time.Time  `bson:"occurred_at"`
	ProcessedAt *time.Time `bson:"processed_at"` // null until the relay has handled it
}

// todoViewDoc is the BSON shape of the todo_views read model.
type todoViewDoc struct {
	ID        string    `bson:"_id"`
	Title     string    `bson:"title"`
	Completed bool      `bson:"completed"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
