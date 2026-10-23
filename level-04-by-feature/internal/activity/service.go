package activity

import (
	"context"
	"time"
)

type Service struct {
	activities Repository
}

func NewService(activities Repository) *Service {
	return &Service{activities: activities}
}

// Record appends one activity. It never starts a transaction of its own: when
// the caller is inside WithinTx, ctx carries that transaction and the write
// joins it. at is passed in so the activity carries the exact timestamp of the
// change it describes.
func (s *Service) Record(ctx context.Context, todoID, action string, at time.Time) error {
	a, err := New(todoID, action, at)
	if err != nil {
		return err
	}
	return s.activities.Create(ctx, a)
}

// List does not check that the todo exists: activities outlive deleted todos.
func (s *Service) List(ctx context.Context, todoID string) ([]Activity, error) {
	return s.activities.ListByTodo(ctx, todoID)
}
