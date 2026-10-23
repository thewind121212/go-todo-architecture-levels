// Package outbox is the transactional outbox: the bridge from the write side to everything
// that reacts to its events. TodoRepository.Save writes each domain event as a Message in
// the same transaction as the todo's state; the Relay later hands every unprocessed Message
// to a Handler and marks it processed. Neither the Message nor the Relay knows a database.
package outbox

import "time"

// Message is one row of the outbox: a domain event, serialized, waiting for the relay.
type Message struct {
	ID         string // the event id (UUIDv7): time-ordered, so it is also the relay's order
	TodoID     string
	Name       string // the event's name, which says how to read Payload
	Payload    []byte // the event's own fields as JSON
	OccurredAt time.Time
}
