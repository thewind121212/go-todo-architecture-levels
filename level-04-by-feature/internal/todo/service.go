package todo

import "context"

// ActivityRecorder is the one thing this feature needs from the activity
// feature. The interface is declared here, by its consumer, so this package
// never imports activity: activity.Service satisfies it implicitly and main
// wires the two together. Tests plug in a hand-written fake.
type ActivityRecorder interface {
	Record(ctx context.Context, todoID, action string) error
}

// TxManager runs fn in one transaction. platform/tx implements it for both
// databases.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	repo       Repository
	tx         TxManager
	activities ActivityRecorder
}

func NewService(repo Repository, tx TxManager, activities ActivityRecorder) *Service {
	return &Service{repo: repo, tx: tx, activities: activities}
}

func (s *Service) Create(ctx context.Context, title string) (Todo, error) {
	t, err := New(title)
	if err != nil {
		return Todo{}, err
	}
	// One transaction spans two features: the todo row and its activity row
	// both join it through ctx.
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, t); err != nil {
			return err
		}
		return s.activities.Record(ctx, t.ID, ActionCreated)
	})
	if err != nil {
		return Todo{}, err
	}
	return t, nil
}

func (s *Service) Get(ctx context.Context, id string) (Todo, error) {
	id, err := parseID(id)
	if err != nil {
		return Todo{}, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	if limit < 1 || limit > maxLimit {
		return nil, ErrInvalidLimit
	}
	if offset < 0 {
		return nil, ErrInvalidOffset
	}
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	if p.Title == nil && p.Completed == nil {
		return Todo{}, ErrEmptyUpdate
	}
	id, err := parseID(id)
	if err != nil {
		return Todo{}, err
	}
	var updated Todo
	// The read happens inside the transaction because the activities written
	// depend on the current state.
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		t, err := s.repo.Get(ctx, id)
		if err != nil {
			return err
		}
		actions, err := t.Apply(p)
		if err != nil {
			return err
		}
		updated = t
		if len(actions) == 0 {
			return nil
		}
		if err := s.repo.Update(ctx, t); err != nil {
			return err
		}
		for _, action := range actions {
			if err := s.activities.Record(ctx, t.ID, action); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id, err := parseID(id)
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
		return s.activities.Record(ctx, id, ActionDeleted)
	})
}
