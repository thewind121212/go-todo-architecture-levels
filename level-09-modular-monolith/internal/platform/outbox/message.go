// Package outbox is the transactional outbox, as shared platform code: a module's repository
// writes each event as a Message in the same transaction as the state it describes; the
// Relay later hands every unprocessed Message to the module's Handler and marks it processed.
// Neither the Message nor the Relay knows a database, a module or an event type.
package outbox

import "time"

// Message is one row of a module's outbox: an event, serialized, waiting for the relay.
type Message struct {
	ID          string // the event id (UUIDv7): time-ordered, so it is also the relay's order
	AggregateID string // which aggregate the event is about (in the todo module, the todo id)
	Name        string // the event's name, which says how to read Payload
	Payload     []byte // the event's own fields as JSON
	OccurredAt  time.Time
}
