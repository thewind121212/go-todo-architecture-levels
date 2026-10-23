// Package codec is the todo module's outbox format: how a domain event becomes an
// outbox.Message (Encode, called by TodoRepository.Save) and back (Decode, called by the
// publisher). The format is private to the module; other modules see the contracts instead.
package codec

import (
	"encoding/json"
	"fmt"

	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/outbox"
)

// titlePayload is the JSON of the two events that carry data: {"title":"..."}.
// The other events carry nothing but their metadata, which has its own columns: {}.
type titlePayload struct {
	Title string `json:"title"`
}

// Encode turns a domain event into a Message. The mapping is spelled out per type, not
// derived by reflection, so the stored JSON is a deliberate format that a rename in the
// domain cannot change by accident.
func Encode(e domain.Event) (outbox.Message, error) {
	var payload any
	switch e := e.(type) {
	case domain.TodoCreated:
		payload = titlePayload{Title: e.Title}
	case domain.TodoRenamed:
		payload = titlePayload{Title: e.Title}
	case domain.TodoCompleted, domain.TodoReopened, domain.TodoDeleted:
		payload = struct{}{}
	default:
		return outbox.Message{}, fmt.Errorf("encode event: unknown type %T", e)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return outbox.Message{}, fmt.Errorf("encode %s event: %w", e.Name(), err)
	}
	meta := e.Meta()
	return outbox.Message{ID: meta.ID, AggregateID: meta.TodoID, Name: string(e.Name()), Payload: data, OccurredAt: meta.OccurredAt}, nil
}

// Decode is the inverse of Encode: the Message's name picks the event type to rebuild.
func Decode(m outbox.Message) (domain.Event, error) {
	meta := domain.EventMeta{ID: m.ID, TodoID: m.AggregateID, OccurredAt: m.OccurredAt}
	switch domain.Action(m.Name) {
	case domain.ActionCreated:
		var p titlePayload
		if err := decodePayload(m, &p); err != nil {
			return nil, err
		}
		return domain.TodoCreated{EventMeta: meta, Title: p.Title}, nil
	case domain.ActionRenamed:
		var p titlePayload
		if err := decodePayload(m, &p); err != nil {
			return nil, err
		}
		return domain.TodoRenamed{EventMeta: meta, Title: p.Title}, nil
	case domain.ActionCompleted:
		return domain.TodoCompleted{EventMeta: meta}, nil
	case domain.ActionReopened:
		return domain.TodoReopened{EventMeta: meta}, nil
	case domain.ActionDeleted:
		return domain.TodoDeleted{EventMeta: meta}, nil
	default:
		return nil, fmt.Errorf("decode event %s: unknown name %q", m.ID, m.Name)
	}
}

func decodePayload(m outbox.Message, payload any) error {
	if err := json.Unmarshal(m.Payload, payload); err != nil {
		return fmt.Errorf("decode %s event %s: %w", m.Name, m.ID, err)
	}
	return nil
}
