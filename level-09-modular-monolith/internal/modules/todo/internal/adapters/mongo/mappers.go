package mongo

import (
	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/modules/todo/internal/codec"
	"example.com/todo/level09/internal/modules/todo/internal/domain"
	"example.com/todo/level09/internal/platform/outbox"
)

// toTodoDoc is the document a save writes, so its version is the next one after the loaded version.
func toTodoDoc(t *domain.Todo) *todoDoc {
	return &todoDoc{
		ID:        t.ID().String(),
		Title:     t.Title().String(),
		Completed: t.Completed(),
		CreatedAt: t.CreatedAt(),
		UpdatedAt: t.UpdatedAt(),
		Version:   t.Version() + 1,
	}
}

func toDomainTodo(d *todoDoc) *domain.Todo {
	return domain.RehydrateTodo(d.ID, d.Title, d.Completed, d.CreatedAt.UTC(), d.UpdatedAt.UTC(), d.Version)
}

// toOutboxDocs serializes domain events into outbox documents: the event id is the document id.
func toOutboxDocs(events []domain.Event) ([]outboxDoc, error) {
	docs := make([]outboxDoc, 0, len(events))
	for _, e := range events {
		m, err := codec.Encode(e)
		if err != nil {
			return nil, err
		}
		docs = append(docs, outboxDoc{
			ID:         m.ID,
			TodoID:     m.AggregateID,
			Name:       m.Name,
			Payload:    string(m.Payload),
			OccurredAt: m.OccurredAt,
		})
	}
	return docs, nil
}

func toMessage(d *outboxDoc) outbox.Message {
	return outbox.Message{
		ID:          d.ID,
		AggregateID: d.TodoID,
		Name:        d.Name,
		Payload:     []byte(d.Payload),
		OccurredAt:  d.OccurredAt.UTC(),
	}
}

func toTodoViewDoc(v *query.TodoView) *todoViewDoc {
	return &todoViewDoc{
		ID:        v.ID,
		Title:     v.Title,
		Completed: v.Completed,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

func toTodoView(d *todoViewDoc) *query.TodoView {
	return &query.TodoView{
		ID:        d.ID,
		Title:     d.Title,
		Completed: d.Completed,
		CreatedAt: d.CreatedAt.UTC(),
		UpdatedAt: d.UpdatedAt.UTC(),
	}
}
