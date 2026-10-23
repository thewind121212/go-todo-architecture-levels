package mongo

import "example.com/todo/level06/internal/todo/domain"

func toTodoDoc(t *domain.Todo) *todoDoc {
	return &todoDoc{
		ID:        t.ID,
		Title:     t.Title,
		Completed: t.Completed,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toDomainTodo(d *todoDoc) *domain.Todo {
	return &domain.Todo{
		ID:        d.ID,
		Title:     d.Title,
		Completed: d.Completed,
		CreatedAt: d.CreatedAt.UTC(),
		UpdatedAt: d.UpdatedAt.UTC(),
	}
}

func toActivityDoc(a *domain.Activity) *activityDoc {
	return &activityDoc{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    string(a.Action),
		CreatedAt: a.CreatedAt,
	}
}

func toDomainActivity(d *activityDoc) *domain.Activity {
	return &domain.Activity{
		ID:        d.ID,
		TodoID:    d.TodoID,
		Action:    domain.Action(d.Action),
		CreatedAt: d.CreatedAt.UTC(),
	}
}
