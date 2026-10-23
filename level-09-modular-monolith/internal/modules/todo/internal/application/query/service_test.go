package query_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level09/internal/modules/todo/internal/application/query"
	"example.com/todo/level09/internal/modules/todo/internal/application/query/mocks"
)

// newService wires the service to a fresh mock. No expectation means "must not be called":
// in particular, no test here expects a write, because queries only read.
func newService(t *testing.T) (*query.Service, *mocks.MockTodoViews) {
	todos := mocks.NewMockTodoViews(t)
	return query.NewService(todos), todos
}

func TestGetTodoPassesTheCanonicalID(t *testing.T) {
	svc, todos := newService(t)
	view := &query.TodoView{ID: uuid.NewString(), Title: "buy milk"}
	todos.EXPECT().Get(mock.Anything, view.ID).Return(view, nil).Once()

	got, err := svc.GetTodo(context.Background(), strings.ToUpper(view.ID))

	require.NoError(t, err)
	require.Equal(t, view, got)
}

// A malformed id names no todo: it is ErrNotFound, and the views are never asked.
func TestMalformedIDIsNotFound(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.GetTodo(context.Background(), "not-a-uuid")
	require.ErrorIs(t, err, query.ErrNotFound)
}

func TestListTodosPassesThePage(t *testing.T) {
	svc, todos := newService(t)
	views := []*query.TodoView{{ID: uuid.NewString()}}
	todos.EXPECT().List(mock.Anything, 5, 10).Return(views, nil).Once()

	got, err := svc.ListTodos(context.Background(), 5, 10)

	require.NoError(t, err)
	require.Equal(t, views, got)
}
