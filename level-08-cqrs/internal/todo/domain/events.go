package domain

import "time"

// Event is a fact the Todo aggregate recorded: something that happened to a todo.
// The repository writes each event to the outbox; its Name tells the reader how to decode it.
// Events are plain data with primitive fields, so they serialize easily.
type Event interface {
	Name() Action
	Meta() EventMeta
}

// EventMeta is what every event carries. ID is the outbox row's id, and later the activity's id.
type EventMeta struct {
	ID         string // UUIDv7
	TodoID     string
	OccurredAt time.Time
}

// Meta is promoted to every event that embeds EventMeta.
func (m EventMeta) Meta() EventMeta { return m }

type TodoCreated struct {
	EventMeta
	Title string
}

type TodoRenamed struct {
	EventMeta
	Title string // the new title
}

type TodoCompleted struct{ EventMeta }

type TodoReopened struct{ EventMeta }

type TodoDeleted struct{ EventMeta }

func (TodoCreated) Name() Action   { return ActionCreated }
func (TodoRenamed) Name() Action   { return ActionRenamed }
func (TodoCompleted) Name() Action { return ActionCompleted }
func (TodoReopened) Name() Action  { return ActionReopened }
func (TodoDeleted) Name() Action   { return ActionDeleted }

func newEventMeta(id TodoID, now time.Time) EventMeta {
	return EventMeta{ID: newID(), TodoID: id.String(), OccurredAt: now}
}
