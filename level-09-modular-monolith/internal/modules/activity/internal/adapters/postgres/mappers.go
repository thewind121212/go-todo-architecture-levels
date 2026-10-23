package postgres

import "example.com/todo/level09/internal/modules/activity/internal/application"

func toActivityModel(a *application.Activity) *activityModel {
	return &activityModel{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    a.Action,
		CreatedAt: a.CreatedAt,
	}
}

// toActivity converts the time to UTC: pgx scans timestamptz in the local zone.
func toActivity(m *activityModel) *application.Activity {
	return &application.Activity{
		ID:        m.ID,
		TodoID:    m.TodoID,
		Action:    m.Action,
		CreatedAt: m.CreatedAt.UTC(),
	}
}
