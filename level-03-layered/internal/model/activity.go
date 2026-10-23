package model

import "time"

// Action says what happened to a todo.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Activity is one entry in a todo's append-only audit log. It has no foreign
// key on purpose: activities outlive the todo they describe.
type Activity struct {
	ID        string
	TodoID    string
	Action    Action
	CreatedAt time.Time
}

// NewActivity returns a new activity for todoID, stamped with the time of the
// change it records.
func NewActivity(todoID string, action Action, at time.Time) Activity {
	return Activity{ID: newID(), TodoID: todoID, Action: action, CreatedAt: at}
}
