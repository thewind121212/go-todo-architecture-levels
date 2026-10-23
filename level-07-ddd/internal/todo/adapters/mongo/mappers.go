package mongo

import "example.com/todo/level07/internal/todo/domain"

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

// toActivityDocs turns domain events into activity documents: the event id is the activity id.
func toActivityDocs(events []domain.Event) []activityDoc {
	docs := make([]activityDoc, 0, len(events))
	for _, e := range events {
		meta := e.Meta()
		docs = append(docs, activityDoc{
			ID:        meta.ID,
			TodoID:    meta.TodoID,
			Action:    string(e.Name()),
			CreatedAt: meta.OccurredAt,
		})
	}
	return docs
}

func toDomainActivity(d *activityDoc) *domain.Activity {
	return &domain.Activity{
		ID:        d.ID,
		TodoID:    d.TodoID,
		Action:    domain.Action(d.Action),
		CreatedAt: d.CreatedAt.UTC(),
	}
}
