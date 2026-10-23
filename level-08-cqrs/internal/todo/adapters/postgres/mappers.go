package postgres

import (
	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/outbox"
)

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

// toOutboxModels serializes domain events into outbox rows: the event id is the row id.
func toOutboxModels(events []domain.Event) ([]outboxModel, error) {
	models := make([]outboxModel, 0, len(events))
	for _, e := range events {
		m, err := outbox.Encode(e)
		if err != nil {
			return nil, err
		}
		models = append(models, outboxModel{
			ID:         m.ID,
			TodoID:     m.TodoID,
			Name:       m.Name,
			Payload:    m.Payload,
			OccurredAt: m.OccurredAt,
		})
	}
	return models, nil
}

func toMessage(m *outboxModel) outbox.Message {
	return outbox.Message{
		ID:         m.ID,
		TodoID:     m.TodoID,
		Name:       m.Name,
		Payload:    m.Payload,
		OccurredAt: m.OccurredAt.UTC(),
	}
}

func toTodoViewModel(v *query.TodoView) *todoViewModel {
	return &todoViewModel{
		ID:        v.ID,
		Title:     v.Title,
		Completed: v.Completed,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

func toTodoView(m *todoViewModel) *query.TodoView {
	return &query.TodoView{
		ID:        m.ID,
		Title:     m.Title,
		Completed: m.Completed,
		CreatedAt: m.CreatedAt.UTC(),
		UpdatedAt: m.UpdatedAt.UTC(),
	}
}

func toActivityViewModel(a *query.ActivityView) *activityViewModel {
	return &activityViewModel{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    a.Action,
		CreatedAt: a.CreatedAt,
	}
}

func toActivityView(m *activityViewModel) *query.ActivityView {
	return &query.ActivityView{
		ID:        m.ID,
		TodoID:    m.TodoID,
		Action:    m.Action,
		CreatedAt: m.CreatedAt.UTC(),
	}
}
