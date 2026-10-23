package domain

import "time"

// Action names what happened to a todo. It is also the name of the event that recorded it.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Activity is a recorded domain event, as the audit log shows it. It outlives the todo
// it describes. Only repositories create activities, from the events they save.
type Activity struct {
	ID        string // the event's id
	TodoID    string
	Action    Action
	CreatedAt time.Time // when the event occurred
}
