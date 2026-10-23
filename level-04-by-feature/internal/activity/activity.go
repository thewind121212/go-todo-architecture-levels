// Package activity is the append-only audit log of what happened to a todo.
// It knows nothing about the todo feature: it never imports it, and it treats
// todo ids and actions as plain strings.
package activity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Activity has no foreign key to its todo: activities outlive deleted todos.
type Activity struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	TodoID    string    `json:"todo_id" gorm:"type:uuid;not null;index" bson:"todo_id"`
	Action    string    `json:"action" gorm:"not null" bson:"action"`
	CreatedAt time.Time `json:"created_at" gorm:"not null" bson:"created_at"`
}

func New(todoID, action string) (Activity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Activity{}, fmt.Errorf("new activity id: %w", err)
	}
	// Truncated to milliseconds because Mongo stores milliseconds.
	now := time.Now().UTC().Truncate(time.Millisecond)
	return Activity{ID: id.String(), TodoID: todoID, Action: action, CreatedAt: now}, nil
}
