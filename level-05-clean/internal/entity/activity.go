package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Action names what happened to a todo.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Activity is one entry in a todo's append-only audit log. TodoID is not a foreign key:
// activities outlive deleted todos.
type Activity struct {
	ID        string
	TodoID    string
	Action    Action
	CreatedAt time.Time
}

// NewActivity records that action happened to the todo with the given id at now.
func NewActivity(todoID string, action Action, now time.Time) (Activity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Activity{}, fmt.Errorf("new activity id: %w", err)
	}
	return Activity{ID: id.String(), TodoID: todoID, Action: action, CreatedAt: now}, nil
}
