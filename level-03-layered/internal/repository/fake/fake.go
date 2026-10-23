// Package fake has hand-written in-memory repositories and a TxManager for
// unit tests of the service and the handler. They are not safe for
// concurrent use, and the fake TxManager does not undo writes: it only
// records whether the transaction would have committed or rolled back.
package fake

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"example.com/todo/level03/internal/model"
)

type TodoRepository struct {
	todos map[string]model.Todo
}

func NewTodoRepository() *TodoRepository {
	return &TodoRepository{todos: map[string]model.Todo{}}
}

func (r *TodoRepository) Create(_ context.Context, todo model.Todo) error {
	r.todos[todo.ID] = todo
	return nil
}

func (r *TodoRepository) Get(_ context.Context, id string) (model.Todo, error) {
	todo, ok := r.todos[id]
	if !ok {
		return model.Todo{}, model.ErrNotFound
	}
	return todo, nil
}

// GetForUpdate is Get: the fakes run one request at a time, so there is nothing to lock.
func (r *TodoRepository) GetForUpdate(ctx context.Context, id string) (model.Todo, error) {
	return r.Get(ctx, id)
}

func (r *TodoRepository) List(_ context.Context, limit, offset int) ([]model.Todo, error) {
	todos := slices.Collect(maps.Values(r.todos))
	slices.SortFunc(todos, func(a, b model.Todo) int { return cmp.Compare(b.ID, a.ID) })
	if offset >= len(todos) {
		return nil, nil
	}
	return todos[offset:min(offset+limit, len(todos))], nil
}

func (r *TodoRepository) Update(_ context.Context, todo model.Todo) error {
	if _, ok := r.todos[todo.ID]; !ok {
		return model.ErrNotFound
	}
	r.todos[todo.ID] = todo
	return nil
}

func (r *TodoRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.todos[id]; !ok {
		return model.ErrNotFound
	}
	delete(r.todos, id)
	return nil
}

// ActivityRepository keeps activities in insertion order. Set Err to make
// Create fail.
type ActivityRepository struct {
	Activities []model.Activity
	Err        error
}

func (r *ActivityRepository) Create(_ context.Context, a model.Activity) error {
	if r.Err != nil {
		return r.Err
	}
	r.Activities = append(r.Activities, a)
	return nil
}

func (r *ActivityRepository) ListByTodo(_ context.Context, todoID string) ([]model.Activity, error) {
	var out []model.Activity
	for _, a := range r.Activities {
		if a.TodoID == todoID {
			out = append(out, a)
		}
	}
	return out, nil
}

// TxManager runs fn directly and counts how each transaction ended.
type TxManager struct {
	Commits   int
	Rollbacks int
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := fn(ctx); err != nil {
		m.Rollbacks++
		return err
	}
	m.Commits++
	return nil
}
