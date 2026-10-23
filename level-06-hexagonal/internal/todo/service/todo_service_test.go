package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
	"example.com/todo/level06/internal/todo/ports/mocks"
	"example.com/todo/level06/internal/todo/service"
)

var errBoom = errors.New("boom")

type txKey struct{}

type deps struct {
	todos      *mocks.MockTodoRepository
	activities *mocks.MockActivityRepository
	tx         *mocks.MockTxManager
}

// newService wires the service to fresh mocks. A mock fails the test on any
// call it was not told to expect, so "no expectation" means "must not be called".
func newService(t *testing.T) (*service.TodoService, deps) {
	d := deps{
		todos:      mocks.NewMockTodoRepository(t),
		activities: mocks.NewMockActivityRepository(t),
		tx:         mocks.NewMockTxManager(t),
	}
	return service.NewTodoService(d.todos, d.activities, d.tx), d
}

// expectTx makes the mock TxManager run fn the way a real one does, with a
// marked ctx. Repository expectations use inTx to check they ran inside it.
func expectTx(tx *mocks.MockTxManager) {
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(context.WithValue(ctx, txKey{}, true))
		}).Once()
}

func inTx() any {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txKey{}) != nil })
}

func action(want domain.Action) any {
	return mock.MatchedBy(func(a *domain.Activity) bool { return a.Action == want })
}

func existingTodo(t *testing.T, completed bool) *domain.Todo {
	t.Helper()
	todo, err := domain.NewTodo("old", time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)
	todo.Completed = completed
	return todo
}

func TestCreate(t *testing.T) {
	svc, d := newService(t)
	var activity *domain.Activity
	expectTx(d.tx)
	d.todos.EXPECT().Create(inTx(), mock.Anything).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), action(domain.ActionCreated)).
		RunAndReturn(func(_ context.Context, a *domain.Activity) error {
			activity = a
			return nil
		}).Once()

	todo, err := svc.Create(context.Background(), "  buy milk ")

	require.NoError(t, err)
	require.Equal(t, "buy milk", todo.Title)
	require.Equal(t, todo.ID, activity.TodoID)
	require.Equal(t, todo.CreatedAt, activity.CreatedAt, "one now per operation")
}

func TestCreateInvalidTitleTouchesNothing(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Create(context.Background(), " ")

	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestCreateActivityFailureIsReturned(t *testing.T) {
	svc, d := newService(t)
	expectTx(d.tx)
	d.todos.EXPECT().Create(inTx(), mock.Anything).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), mock.Anything).Return(errBoom).Once()

	_, err := svc.Create(context.Background(), "buy milk")

	// The error leaves the tx callback, so a real TxManager rolls the todo back.
	require.ErrorIs(t, err, errBoom)
}

func TestUpdateTitleAndCompletedRecordsBothInOrder(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	var recorded []*domain.Activity
	expectTx(d.tx)
	d.todos.EXPECT().GetForUpdate(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), mock.Anything).
		RunAndReturn(func(_ context.Context, a *domain.Activity) error {
			recorded = append(recorded, a)
			return nil
		}).Twice()

	got, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID, Title: new("new"), Completed: new(true)})

	require.NoError(t, err)
	require.Equal(t, "new", got.Title)
	require.True(t, got.Completed)
	require.Len(t, recorded, 2)
	require.Equal(t, domain.ActionRenamed, recorded[0].Action)
	require.Equal(t, domain.ActionCompleted, recorded[1].Action)
	for _, a := range recorded {
		require.Equal(t, got.UpdatedAt, a.CreatedAt, "one now per operation")
	}
}

func TestUpdateReopen(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, true)
	expectTx(d.tx)
	d.todos.EXPECT().GetForUpdate(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), action(domain.ActionReopened)).Return(nil).Once()

	got, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID, Completed: new(false)})

	require.NoError(t, err)
	require.False(t, got.Completed)
}

func TestUpdateWithoutChangeWritesNothing(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	expectTx(d.tx)
	d.todos.EXPECT().GetForUpdate(inTx(), todo.ID).Return(todo, nil).Once()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID, Title: new("old"), Completed: new(false)})

	require.NoError(t, err)
}

func TestUpdateActivityFailureIsReturned(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	expectTx(d.tx)
	d.todos.EXPECT().GetForUpdate(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), mock.Anything).Return(errBoom).Once()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: todo.ID, Completed: new(true)})

	require.ErrorIs(t, err, errBoom)
}

// Bad input is rejected before WithinTx: no expectations are set, so any
// repository or TxManager call would fail the test.
func TestUpdateRejectsBadInputBeforeTx(t *testing.T) {
	svc, _ := newService(t)
	id := uuid.NewString()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: id})
	require.ErrorIs(t, err, domain.ErrEmptyPatch)

	_, err = svc.Update(context.Background(), ports.UpdateTodoInput{ID: id, Title: new(" ")})
	require.ErrorIs(t, err, domain.ErrInvalidTitle)
}

func TestUpdateUnknownID(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().GetForUpdate(inTx(), id).Return(nil, domain.ErrNotFound).Once()

	_, err := svc.Update(context.Background(), ports.UpdateTodoInput{ID: id, Completed: new(true)})

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDelete(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().Delete(inTx(), id).Return(nil).Once()
	d.activities.EXPECT().Create(inTx(), action(domain.ActionDeleted)).Return(nil).Once()

	require.NoError(t, svc.Delete(context.Background(), id))
}

func TestDeleteUnknownIDRecordsNothing(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().Delete(inTx(), id).Return(domain.ErrNotFound).Once()

	require.ErrorIs(t, svc.Delete(context.Background(), id), domain.ErrNotFound)
}
