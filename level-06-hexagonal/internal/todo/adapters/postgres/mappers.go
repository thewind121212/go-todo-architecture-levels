package postgres

import "example.com/todo/level06/internal/todo/domain"

func toTodoModel(t *domain.Todo) *todoModel {
	return &todoModel{
		ID:        t.ID,
		Title:     t.Title,
		Completed: t.Completed,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// toDomainTodo converts times to UTC: pgx scans timestamptz in the local zone.
func toDomainTodo(m *todoModel) *domain.Todo {
	return &domain.Todo{
		ID:        m.ID,
		Title:     m.Title,
		Completed: m.Completed,
		CreatedAt: m.CreatedAt.UTC(),
		UpdatedAt: m.UpdatedAt.UTC(),
	}
}

func toActivityModel(a *domain.Activity) *activityModel {
	return &activityModel{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    string(a.Action),
		CreatedAt: a.CreatedAt,
	}
}

func toDomainActivity(m *activityModel) *domain.Activity {
	return &domain.Activity{
		ID:        m.ID,
		TodoID:    m.TodoID,
		Action:    domain.Action(m.Action),
		CreatedAt: m.CreatedAt.UTC(),
	}
}
