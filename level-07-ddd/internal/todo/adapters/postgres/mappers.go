package postgres

import "example.com/todo/level07/internal/todo/domain"

// toTodoModel is the row a save writes, so its version is the next one after the loaded version.
func toTodoModel(t *domain.Todo) *todoModel {
	return &todoModel{
		ID:        t.ID().String(),
		Title:     t.Title().String(),
		Completed: t.Completed(),
		CreatedAt: t.CreatedAt(),
		UpdatedAt: t.UpdatedAt(),
		Version:   t.Version() + 1,
	}
}

// toDomainTodo converts times to UTC: pgx scans timestamptz in the local zone.
func toDomainTodo(m *todoModel) *domain.Todo {
	return domain.RehydrateTodo(m.ID, m.Title, m.Completed, m.CreatedAt.UTC(), m.UpdatedAt.UTC(), m.Version)
}

// toActivityModels turns domain events into activity rows: the event id is the activity id.
func toActivityModels(events []domain.Event) []activityModel {
	models := make([]activityModel, 0, len(events))
	for _, e := range events {
		meta := e.Meta()
		models = append(models, activityModel{
			ID:        meta.ID,
			TodoID:    meta.TodoID,
			Action:    string(e.Name()),
			CreatedAt: meta.OccurredAt,
		})
	}
	return models
}

func toDomainActivity(m *activityModel) *domain.Activity {
	return &domain.Activity{
		ID:        m.ID,
		TodoID:    m.TodoID,
		Action:    domain.Action(m.Action),
		CreatedAt: m.CreatedAt.UTC(),
	}
}
