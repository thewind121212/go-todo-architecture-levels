// Package mongo is a driven adapter: it implements domain.TodoRepository and
// ports.ActivityLog with the official MongoDB driver.
package mongo

import "time"

// Collection names. Collections and indexes are created at startup
// by internal/infrastructure/database.
const (
	todosCollection      = "todos"
	activitiesCollection = "activities"
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

// activityDoc is the BSON shape of an activity: one stored domain event.
type activityDoc struct {
	ID        string    `bson:"_id"`
	TodoID    string    `bson:"todo_id"`
	Action    string    `bson:"action"`
	CreatedAt time.Time `bson:"created_at"`
}
