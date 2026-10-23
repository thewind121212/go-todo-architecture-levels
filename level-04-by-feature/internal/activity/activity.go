// Package activity is the append-only audit log of what happened to a todo.
// It knows nothing about the todo feature: it never imports it, and it treats
// todo ids and actions as plain strings.
package activity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Activity is one entry in a todo's audit log. TodoID is not a foreign key:
// activities outlive deleted todos.
type Activity struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time
}

// New returns a new activity for todoID that happened at now.
func New(todoID, action string, now time.Time) (Activity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Activity{}, fmt.Errorf("new activity id: %w", err)
	}
	return Activity{ID: id.String(), TodoID: todoID, Action: action, CreatedAt: now}, nil
}
