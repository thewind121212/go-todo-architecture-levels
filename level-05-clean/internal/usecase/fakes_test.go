package usecase_test

import (
	"context"
	"errors"

	"example.com/todo/level05/internal/entity"
)

var errBoom = errors.New("boom")

// fakeTodos is an in-memory TodoRepository. Set err to make every call fail.
type fakeTodos struct {
	items      map[string]entity.Todo
	updates    int
	listLimit  int
	listOffset int
	err        error
}

func newFakeTodos(todos ...entity.Todo) *fakeTodos {
	f := &fakeTodos{items: map[string]entity.Todo{}}
	for _, t := range todos {
		f.items[t.ID] = t
	}
	return f
}

func (f *fakeTodos) Create(_ context.Context, todo entity.Todo) error {
	if f.err != nil {
		return f.err
	}
	f.items[todo.ID] = todo
	return nil
}

func (f *fakeTodos) FindByID(_ context.Context, id string) (entity.Todo, error) {
	if f.err != nil {
		return entity.Todo{}, f.err
	}
	todo, ok := f.items[id]
	if !ok {
		return entity.Todo{}, entity.ErrNotFound
	}
	return todo, nil
}

func (f *fakeTodos) List(_ context.Context, limit, offset int) ([]entity.Todo, error) {
	f.listLimit, f.listOffset = limit, offset
	if f.err != nil {
		return nil, f.err
	}
	var todos []entity.Todo
	for _, t := range f.items {
		todos = append(todos, t)
	}
	return todos, nil
}

func (f *fakeTodos) Update(_ context.Context, todo entity.Todo) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.items[todo.ID]; !ok {
		return entity.ErrNotFound
	}
	f.updates++
	f.items[todo.ID] = todo
	return nil
}

func (f *fakeTodos) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.items[id]; !ok {
		return entity.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

// fakeActivities is an in-memory ActivityRepository. Set err to make every call fail.
type fakeActivities struct {
	items []entity.Activity
	err   error
}

func (f *fakeActivities) Create(_ context.Context, activity entity.Activity) error {
	if f.err != nil {
		return f.err
	}
	f.items = append(f.items, activity)
	return nil
}

func (f *fakeActivities) ListByTodoID(_ context.Context, todoID string) ([]entity.Activity, error) {
	if f.err != nil {
		return nil, f.err
	}
	var activities []entity.Activity
	for _, a := range f.items {
		if a.TodoID == todoID {
			activities = append(activities, a)
		}
	}
	return activities, nil
}

func (f *fakeActivities) actions() []entity.Action {
	var actions []entity.Action
	for _, a := range f.items {
		actions = append(actions, a.Action)
	}
	return actions
}

// fakeTx runs fn directly and remembers whether fn failed, which is when a real TxManager rolls back.
// The fakes themselves cannot roll back; the integration tests prove that the real adapters do.
type fakeTx struct {
	calls  int
	failed bool
}

func (f *fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	err := fn(ctx)
	f.failed = err != nil
	return err
}

func mustNewTodo(title string) entity.Todo {
	todo, err := entity.NewTodo(title)
	if err != nil {
		panic(err)
	}
	return todo
}
