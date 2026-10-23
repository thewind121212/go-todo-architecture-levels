package mongo

import "example.com/todo/level09/internal/modules/activity/internal/application"

func toActivityDoc(a *application.Activity) *activityDoc {
	return &activityDoc{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    a.Action,
		CreatedAt: a.CreatedAt,
	}
}

func toActivity(d *activityDoc) *application.Activity {
	return &application.Activity{
		ID:        d.ID,
		TodoID:    d.TodoID,
		Action:    d.Action,
		CreatedAt: d.CreatedAt.UTC(),
	}
}
