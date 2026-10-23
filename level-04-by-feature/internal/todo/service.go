package todo

import (
	"context"
	"time"
)

// ActivityRecorder is the one thing this feature needs from the activity
// feature. The interface is declared here, by its consumer, so this package
// never imports activity: *activity.Service satisfies it implicitly and main
// wires the two together. Tests plug in a hand-written fake.
type ActivityRecorder interface {
	Record(ctx context.Context, todoID, action string, at time.Time) error
}

// TxManager runs fn in one transaction; fn must use the ctx it receives.
// platform/tx implements it for both databases.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Service holds the business flows. It is the only place that knows which
// writes belong together, so it is the one that opens transactions.
type Service struct {
	tx         TxManager
	todos      Repository
	activities ActivityRecorder
}

func NewService(tx TxManager, todos Repository, activities ActivityRecorder) *Service {
	return &Service{tx: tx, todos: todos, activities: activities}
}

func (s *Service) Create(ctx context.Context, title string) (Todo, error) {
	now := nowUTC()
	todo, err := New(title, now)
	if err != nil {
		return Todo{}, err
	}
	// One transaction spans two features: the todo row and its activity row
	// both join it through ctx.
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Create(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, now, ActionCreated)
	})
	if err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *Service) Get(ctx context.Context, id string) (Todo, error) {
	return s.todos.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	return s.todos.List(ctx, limit, offset)
}

// Update validates the patch first, then reads, applies, saves and records
// one activity per change in one transaction: if any step fails, nothing is
// written.
func (s *Service) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	p, err := ValidatePatch(p)
	if err != nil {
		return Todo{}, err
	}
	var todo Todo
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = s.todos.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		now := nowUTC()
		actions := todo.Apply(p, now)
		if len(actions) == 0 {
			return nil // nothing changed, so nothing to write
		}
		if err := s.todos.Update(ctx, todo); err != nil {
			return err
		}
		return s.record(ctx, todo.ID, now, actions...)
	})
	if err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	now := nowUTC()
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.todos.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, id, now, ActionDeleted)
	})
}

// record hands one activity per action to the activity feature. It must be
// called inside WithinTx with the transaction's ctx, so the activity joins it.
func (s *Service) record(ctx context.Context, todoID string, now time.Time, actions ...Action) error {
	for _, action := range actions {
		if err := s.activities.Record(ctx, todoID, string(action), now); err != nil {
			return err
		}
	}
	return nil
}

// nowUTC is truncated to milliseconds because that is all MongoDB stores; this
// way both adapters return exactly the time that was written.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
