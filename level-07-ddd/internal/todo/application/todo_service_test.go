package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level07/internal/todo/application"
	"example.com/todo/level07/internal/todo/domain"
	domainmocks "example.com/todo/level07/internal/todo/domain/mocks"
	"example.com/todo/level07/internal/todo/ports"
	"example.com/todo/level07/internal/todo/ports/mocks"
)

var errBoom = errors.New("boom")

type deps struct {
	todos      *domainmocks.MockTodoRepository
	activities *mocks.MockActivityLog
}

// newService wires the service to fresh mocks. A mock fails the test on any
// call it was not told to expect, so "no expectation" means "must not be called".
func newService(t *testing.T) (*application.TodoService, deps) {
	d := deps{
		todos:      domainmocks.NewMockTodoRepository(t),
		activities: mocks.NewMockActivityLog(t),
	}
	return application.NewTodoService(d.todos, d.activities), d
}

// expectSave makes the mock repository pull the todo's events, as a real Save does,
// and keeps them in saved for the test to check.
func expectSave(todos *domainmocks.MockTodoRepository, saved *[]domain.Event) {
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

func TestCreate(t *testing.T) {
	svc, d := newService(t)
	var saved []domain.Event
	expectSave(d.todos, &saved)

	todo, err := svc.Create(context.Background(), "  buy milk ")

	require.NoError(t, err)
	require.Equal(t, "buy milk", todo.Title().String())
	require.Equal(t, []domain.Action{domain.ActionCreated}, names(saved))
	require.Equal(t, todo.CreatedAt(), saved[0].Meta().OccurredAt, "one now per operation")
}

func TestCreateInvalidTitleTouchesNothing(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Create(context.Background(), " ")

	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestCreateSaveFailureIsReturned(t *testing.T) {
	svc, d := newService(t)
	d.todos.EXPECT().Save(mock.Anything, mock.Anything).Return(errBoom).Once()

	_, err := svc.Create(context.Background(), "buy milk")

	require.ErrorIs(t, err, errBoom)
}

func TestGetParsesTheID(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(false)
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()

	got, err := svc.Get(context.Background(), strings.ToUpper(todo.ID().String()))

	require.NoError(t, err)
	require.Equal(t, todo, got)
}

func TestUpdateTitleAndCompletedRecordsBothInOrder(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(d.todos, &saved)

	got, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Title: new("new"), Completed: new(true)})

	require.NoError(t, err)
	require.Equal(t, "new", got.Title().String())
	require.True(t, got.Completed())
	require.Equal(t, 1, got.Version(), "the loaded version; the repository writes the next one")
	require.Equal(t, []domain.Action{domain.ActionRenamed, domain.ActionCompleted}, names(saved))
	for _, e := range saved {
		require.Equal(t, got.UpdatedAt(), e.Meta().OccurredAt, "one now per operation")
	}
}

func TestUpdateReopen(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(true)
	var saved []domain.Event
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(d.todos, &saved)

	got, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Completed: new(false)})

	require.NoError(t, err)
	require.False(t, got.Completed())
	require.Equal(t, []domain.Action{domain.ActionReopened}, names(saved))
}

// Save is still called, but with no events, so the repository writes nothing.
func TestUpdateWithoutChangeRecordsNothing(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(d.todos, &saved)

	got, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Title: new("old"), Completed: new(false)})

	require.NoError(t, err)
	require.Empty(t, saved)
	require.Equal(t, 1, got.Version())
}

// Bad input fails while the value objects are built, before anything is loaded:
// no expectations are set, so any repository call would fail the test.
func TestUpdateRejectsBadInputBeforeLoading(t *testing.T) {
	svc, _ := newService(t)
	id := uuid.NewString()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: "not-a-uuid", Completed: new(true)})
	require.ErrorIs(t, err, domain.ErrInvalidID)

	_, err = svc.Update(context.Background(), ports.UpdateTodoInput{ID: id})
	require.ErrorIs(t, err, domain.ErrEmptyPatch)

	_, err = svc.Update(context.Background(), ports.UpdateTodoInput{ID: id, Title: new(" ")})
	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestUpdateUnknownID(t *testing.T) {
	svc, d := newService(t)
	id := domain.NewTodoID()
	d.todos.EXPECT().Get(mock.Anything, id).Return(nil, domain.ErrNotFound).Once()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: id.String(), Completed: new(true)})

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateConflictIsReturned(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(false)
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	d.todos.EXPECT().Save(mock.Anything, todo).Return(domain.ErrConflict).Once()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID().String(), Completed: new(true)})

	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestDelete(t *testing.T) {
	svc, d := newService(t)
	todo := storedTodo(false)
	var saved []domain.Event
	d.todos.EXPECT().Get(mock.Anything, todo.ID()).Return(todo, nil).Once()
	expectSave(d.todos, &saved)

	require.NoError(t, svc.Delete(context.Background(), todo.ID().String()))
	require.True(t, todo.Deleted())
	require.Equal(t, []domain.Action{domain.ActionDeleted}, names(saved))
}

func TestDeleteUnknownIDSavesNothing(t *testing.T) {
	svc, d := newService(t)
	id := domain.NewTodoID()
	d.todos.EXPECT().Get(mock.Anything, id).Return(nil, domain.ErrNotFound).Once()

	require.ErrorIs(t, svc.Delete(context.Background(), id.String()), domain.ErrNotFound)
}

func TestListActivitiesParsesTheID(t *testing.T) {
	svc, d := newService(t)
	id := domain.NewTodoID()
	d.activities.EXPECT().ListByTodo(mock.Anything, id).Return(nil, nil).Once()

	_, err := svc.ListActivities(context.Background(), id.String())
	require.NoError(t, err)

	_, err = svc.ListActivities(context.Background(), "not-a-uuid")
	require.ErrorIs(t, err, domain.ErrInvalidID)
}
