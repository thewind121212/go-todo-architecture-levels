package todo

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"time"
)

var errBoom = errors.New("boom")

type txKey struct{}

// fakeTx marks ctx so the other fakes can check they ran inside WithinTx. It
// does not undo writes.
type fakeTx struct {
	calls int
}

func (f *fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	return fn(context.WithValue(ctx, txKey{}, true))
}

func inTx(ctx context.Context) bool {
	return ctx.Value(txKey{}) != nil
}

type fakeRepo struct {
	todos map[string]Todo
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{todos: map[string]Todo{}}
}

func (r *fakeRepo) Create(_ context.Context, t Todo) error {
	r.todos[t.ID] = t
	return nil
}

func (r *fakeRepo) Get(_ context.Context, id string) (Todo, error) {
	t, ok := r.todos[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return t, nil
}

// GetForUpdate is Get: the fakes run one request at a time, so there is nothing to lock.
func (r *fakeRepo) GetForUpdate(ctx context.Context, id string) (Todo, error) {
	return r.Get(ctx, id)
}

func (r *fakeRepo) List(_ context.Context, limit, offset int) ([]Todo, error) {
	todos := slices.Collect(maps.Values(r.todos))
	slices.SortFunc(todos, func(a, b Todo) int { return cmp.Compare(b.ID, a.ID) })
	if offset >= len(todos) {
		return nil, nil
	}
	return todos[offset:min(offset+limit, len(todos))], nil
}

func (r *fakeRepo) Update(_ context.Context, t Todo) error {
	if _, ok := r.todos[t.ID]; !ok {
		return ErrNotFound
	}
	r.todos[t.ID] = t
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.todos[id]; !ok {
		return ErrNotFound
	}
	delete(r.todos, id)
	return nil
}

type recorded struct {
	todoID string
	action string
	at     time.Time
	inTx   bool
}

// fakeRecorder stands in for the activity feature. Set err to make it fail.
type fakeRecorder struct {
	records []recorded
	err     error
}

func (r *fakeRecorder) Record(ctx context.Context, todoID, action string, at time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, recorded{todoID: todoID, action: action, at: at, inTx: inTx(ctx)})
	return nil
}

func (r *fakeRecorder) actions() []string {
	var actions []string
	for _, rec := range r.records {
		actions = append(actions, rec.action)
	}
	return actions
}
