package domain

// Action names what happened to a todo. It is also the name of the event that recorded it.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)
