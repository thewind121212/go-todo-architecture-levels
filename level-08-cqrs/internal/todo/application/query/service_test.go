package query_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/application/query/mocks"
)

type deps struct {
	todos      *mocks.MockTodoViews
	activities *mocks.MockActivityViews
}

// newService wires the service to fresh mocks. No expectation means "must not be called":
// in particular, no test here expects a write, because queries only read.
func newService(t *testing.T) (*query.Service, deps) {
	d := deps{todos: mocks.NewMockTodoViews(t), activities: mocks.NewMockActivityViews(t)}
	return query.NewService(d.todos, d.activities), d
}

func TestGetTodoPassesTheCanonicalID(t *testing.T) {
	svc, d := newService(t)
	view := &query.TodoView{ID: uuid.NewString(), Title: "buy milk"}
	d.todos.EXPECT().Get(mock.Anything, view.ID).Return(view, nil).Once()

	got, err := svc.GetTodo(context.Background(), strings.ToUpper(view.ID))

	require.NoError(t, err)
	require.Equal(t, view, got)
}

// A malformed id names no todo: it is ErrNotFound, and the views are never asked.
func TestMalformedIDIsNotFound(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.GetTodo(context.Background(), "not-a-uuid")
	require.ErrorIs(t, err, query.ErrNotFound)

	_, err = svc.ListActivities(context.Background(), "not-a-uuid")
	require.ErrorIs(t, err, query.ErrNotFound)
}

func TestListTodosPassesThePage(t *testing.T) {
	svc, d := newService(t)
	views := []*query.TodoView{{ID: uuid.NewString()}}
	d.todos.EXPECT().List(mock.Anything, 5, 10).Return(views, nil).Once()

	got, err := svc.ListTodos(context.Background(), 5, 10)

	require.NoError(t, err)
	require.Equal(t, views, got)
}

func TestListActivitiesPassesTheCanonicalID(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	d.activities.EXPECT().ListByTodo(mock.Anything, id).Return(nil, nil).Once()

	_, err := svc.ListActivities(context.Background(), strings.ToUpper(id))

	require.NoError(t, err)
}
