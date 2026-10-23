package activity

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Record appends one activity. It never starts a transaction of its own: when
// the caller is inside WithinTx, ctx carries that transaction and the write
// joins it. This is how the todo service gets one commit across two features.
func (s *Service) Record(ctx context.Context, todoID, action string) error {
	a, err := New(todoID, action)
	if err != nil {
		return err
	}
	return s.repo.Create(ctx, a)
}

// List returns a todo's activities, oldest first. A malformed id cannot have
// activities, so it gets an empty list like any unknown id.
func (s *Service) List(ctx context.Context, todoID string) ([]Activity, error) {
	id, err := uuid.Parse(todoID)
	if err != nil {
		return []Activity{}, nil
	}
	return s.repo.ListByTodo(ctx, id.String())
}
