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

// Activity is one entry in a todo's append-only audit log.
// It belongs to the todo feature, so it lives in the same domain package.
type Activity struct {
	ID        string
	TodoID    string
	Action    Action
	CreatedAt time.Time
}

// NewActivity records that action happened to the todo with todoID.
func NewActivity(todoID string, action Action) (*Activity, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	return &Activity{ID: id, TodoID: todoID, Action: action, CreatedAt: now()}, nil
}

// CompletionAction is the action recorded when the completed flag changes to completed.
func CompletionAction(completed bool) Action {
	if completed {
		return ActionCompleted
	}
	return ActionReopened
}
