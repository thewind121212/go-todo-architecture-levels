package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/todo/application/command"
	"example.com/todo/level08/internal/todo/domain"
	"example.com/todo/level08/internal/todo/domain/mocks"
	"example.com/todo/level08/internal/todo/ports"
)

var errBoom = errors.New("boom")

// newService wires the service to a fresh mock. A mock fails the test on any
// call it was not told to expect, so "no expectation" means "must not be called".
func newService(t *testing.T) (*command.Service, *mocks.MockTodoRepository) {
	todos := mocks.NewMockTodoRepository(t)
	return command.NewService(todos), todos
}

// expectSave makes the mock repository pull the todo's events, as a real Save does,
// and keeps them in saved for the test to check.
func expectSave(todos *mocks.MockTodoRepository, saved *[]domain.Event) {
	todos.EXPECT().Save(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, todo *domain.Todo) error {
			*saved = todo.PullEvents()
			return nil
		}).Once()
}

// storedTodo is a todo as the repository's Get returns it: version 1, no pending events.
func storedTodo(completed bool) *domain.Todo {
	past := time.Now().UTC().Add(-time.Hour)
	return domain.RehydrateTodo(uuid.NewString(), "old", completed, past, past, 1)
}

func names(events []domain.Event) []domain.Action {
	var names []domain.Action
	for _, e := range events {
		names = append(names, e.Name())
	}
	return names
}

func TestCreateTodo(t *testing.T) {
	svc, todos := newService(t)
	var saved []domain.Event
	expectSave(todos, &saved)

	todo, err := svc.CreateTodo(context.Background(), "  buy milk ")

	require.NoError(t, err)
	require.Equal(t, "buy milk", todo.Title().String())
	require.Equal(t, []domain.Action{domain.ActionCreated}, names(saved))
	require.Equal(t, todo.CreatedAt(), saved[0].Meta().OccurredAt, "one now per operation")
}

func TestCreateTodoInvalidTitleTouchesNothing(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.CreateTodo(context.Background(), " ")

	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestCreateTodoSaveFailureIsReturned(t *testing.T) {
	svc, todos := newService(t)
	todos.EXPECT().Save(mock.Anything, mock.Anything).Return(errBoom).Once()

	_, err := svc.CreateTodo(context.Background(), "buy milk")

	require.ErrorIs(t, err, errBoom)
}

func TestUpdateTodoTitleAndCompletedRecordsBothInOrder(t *testing.T) {
	svc, todos := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(todos, &saved)

	got, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Title: new("new"), Completed: new(true)})

	require.NoError(t, err)
	require.Equal(t, "new", got.Title().String())
	require.True(t, got.Completed())
	require.Equal(t, 1, got.Version(), "the loaded version; the repository writes the next one")
	require.Equal(t, []domain.Action{domain.ActionRenamed, domain.ActionCompleted}, names(saved))
	for _, e := range saved {
		require.Equal(t, got.UpdatedAt(), e.Meta().OccurredAt, "one now per operation")
	}
}

func TestUpdateTodoReopen(t *testing.T) {
	svc, todos := newService(t)
	todo := storedTodo(true)
	var saved []domain.Event
	todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(todos, &saved)

	got, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Completed: new(false)})

	require.NoError(t, err)
	require.False(t, got.Completed())
	require.Equal(t, []domain.Action{domain.ActionReopened}, names(saved))
}

// Save is still called, but with no events, so the repository writes nothing.
func TestUpdateTodoWithoutChangeRecordsNothing(t *testing.T) {
	svc, todos := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(todos, &saved)

	got, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Title: new("old"), Completed: new(false)})

	require.NoError(t, err)
	require.Empty(t, saved)
	require.Equal(t, 1, got.Version())
}

// Bad input fails while the value objects are built, before anything is loaded:
// no expectations are set, so any repository call would fail the test.
func TestUpdateTodoRejectsBadInputBeforeLoading(t *testing.T) {
	svc, _ := newService(t)
	id := uuid.NewString()

	_, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: "not-a-uuid", Completed: new(true)})
	require.ErrorIs(t, err, domain.ErrInvalidID)

	_, err = svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: id})
	require.ErrorIs(t, err, domain.ErrEmptyPatch)

	_, err = svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: id, Title: new(" ")})
	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestUpdateTodoUnknownID(t *testing.T) {
	svc, todos := newService(t)
	id := domain.NewTodoID()
	todos.EXPECT().Get(mock.Anything, id).Return(nil, domain.ErrNotFound).Once()

	_, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: id.String(), Completed: new(true)})

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateTodoConflictIsReturned(t *testing.T) {
	svc, todos := newService(t)
	todo := storedTodo(false)
	todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	todos.EXPECT().Save(mock.Anything, todo).Return(domain.ErrConflict).Once()

	_, err := svc.UpdateTodo(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Completed: new(true)})

	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestDeleteTodo(t *testing.T) {
	svc, todos := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(todos, &saved)

	require.NoError(t, svc.DeleteTodo(context.Background(), todo.ID().String()))
	require.True(t, todo.Deleted())
	require.Equal(t, []domain.Action{domain.ActionDeleted}, names(saved))
}

func TestDeleteTodoUnknownIDSavesNothing(t *testing.T) {
	svc, todos := newService(t)
	id := domain.NewTodoID()
	todos.EXPECT().Get(mock.Anything, id).Return(nil, domain.ErrNotFound).Once()

	require.ErrorIs(t, svc.DeleteTodo(context.Background(), id.String()), domain.ErrNotFound)
}
