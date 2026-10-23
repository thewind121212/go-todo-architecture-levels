// Package contracts is the todo module's published language: the integration events it
// announces on the event bus. It is the only package of the todo module that other modules
// may import (the arch test enforces it), so it imports nothing but the standard library.
//
// These are not the domain events. Those stay inside the module, free to change with the
// model. These are a promise to other modules: plain data, named explicitly, changed only on
// purpose. Adding a field is safe; renaming or removing one breaks a subscriber, so that would
// be a new event (for example "todo.renamed.v2") published next to the old one until every
// subscriber has moved.
package contracts

import "time"

// Event names: the keys subscribers use on the bus.
const (
	TodoCreatedName   = "todo.created"
	TodoRenamedName   = "todo.renamed"
	TodoCompletedName = "todo.completed"
	TodoReopenedName  = "todo.reopened"
	TodoDeletedName   = "todo.deleted"
)

// EventMeta is what every todo event carries.
type EventMeta struct {
	// EventID is unique per event (UUIDv7) and stays the same when an event is delivered
	// again, so subscribers use it to ignore a repeat.
	EventID    string
	TodoID     string
	OccurredAt time.Time
}

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

func (TodoCreated) EventName() string   { return TodoCreatedName }
func (TodoRenamed) EventName() string   { return TodoRenamedName }
func (TodoCompleted) EventName() string { return TodoCompletedName }
func (TodoReopened) EventName() string  { return TodoReopenedName }
func (TodoDeleted) EventName() string   { return TodoDeletedName }
