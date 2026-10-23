package activity

import "context"

const todoID = "0190a0e4-0000-7000-8000-000000000000"

type fakeRepo struct {
	activities []Activity
}

func (r *fakeRepo) Create(_ context.Context, a Activity) error {
	r.activities = append(r.activities, a)
	return nil
}

func (r *fakeRepo) ListByTodo(_ context.Context, todoID string) ([]Activity, error) {
	found := []Activity{}
	for _, a := range r.activities {
		if a.TodoID == todoID {
			found = append(found, a)
		}
	}
	return found, nil
}
