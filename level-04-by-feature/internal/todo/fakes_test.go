package todo

import (
	"context"
	"errors"
	"sort"
)

type txKey struct{}

// fakeTx marks ctx so the other fakes can check they ran inside WithinTx.
type fakeTx struct{}

func (fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
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

func (r *fakeRepo) List(_ context.Context, limit, offset int) ([]Todo, error) {
	todos := []Todo{}
	for _, t := range r.todos {
		todos = append(todos, t)
	}
	sort.Slice(todos, func(i, j int) bool { return todos[i].ID > todos[j].ID })
	if offset >= len(todos) {
		return []Todo{}, nil
	}
	return todos[offset:min(offset+limit, len(todos))], nil
}

func (r *fakeRepo) Update(ctx context.Context, t Todo) error {
	if _, err := r.Get(ctx, t.ID); err != nil {
		return err
	}
	r.todos[t.ID] = t
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, id string) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	delete(r.todos, id)
	return nil
}

type recorded struct {
	todoID string
	action string
	inTx   bool
}

// fakeRecorder stands in for the activity feature. Set err to make it fail.
type fakeRecorder struct {
	records []recorded
	err     error
}

func (r *fakeRecorder) Record(ctx context.Context, todoID, action string) error {
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, recorded{todoID: todoID, action: action, inTx: inTx(ctx)})
	return nil
}

func (r *fakeRecorder) actions() []string {
	var actions []string
	for _, rec := range r.records {
		actions = append(actions, rec.action)
	}
	return actions
}

var errBoom = errors.New("boom")
