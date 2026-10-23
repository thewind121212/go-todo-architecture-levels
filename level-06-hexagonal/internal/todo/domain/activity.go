package domain

import "time"

// Action names what happened to a todo.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Activity is one entry in a todo's append-only audit log. It outlives the todo it describes.
// It belongs to the todo feature, so it lives in the same domain package.
type Activity struct {
	ID        string
	TodoID    string
	Action    Action
	CreatedAt time.Time
}

// NewActivity records that action happened to the todo with the given ID at now.
func NewActivity(todoID string, action Action, now time.Time) *Activity {
	return &Activity{ID: newID(), TodoID: todoID, Action: action, CreatedAt: now}
}
