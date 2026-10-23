package service_test

import (
	"context"
	"errors"
	"testing"

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
	todo, err := domain.NewTodo("old")
	require.NoError(t, err)
	todo.Completed = completed
	return todo
}

func ptr[T any](v T) *T { return &v }

func TestCreate(t *testing.T) {
	svc, d := newService(t)
	expectTx(d.tx)
	d.todos.EXPECT().Create(inTx(), mock.Anything).Return(nil).Once()
	d.activities.EXPECT().Append(inTx(), action(domain.ActionCreated)).Return(nil).Once()

	todo, err := svc.Create(context.Background(), "  buy milk ")

	require.NoError(t, err)
	require.Equal(t, "buy milk", todo.Title)
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
	d.activities.EXPECT().Append(inTx(), mock.Anything).Return(errBoom).Once()

	_, err := svc.Create(context.Background(), "buy milk")

	// The error leaves the tx callback, so a real TxManager rolls the todo back.
	require.ErrorIs(t, err, errBoom)
}

func TestUpdateTitleAndCompletedRecordsBothInOrder(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	var recorded []domain.Action
	expectTx(d.tx)
	d.todos.EXPECT().Get(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Append(inTx(), mock.Anything).
		RunAndReturn(func(_ context.Context, a *domain.Activity) error {
			recorded = append(recorded, a.Action)
			return nil
		}).Twice()

	got, err := svc.Update(context.Background(), todo.ID, ports.UpdateTodoInput{Title: ptr("new"), Completed: ptr(true)})

	require.NoError(t, err)
	require.Equal(t, "new", got.Title)
	require.True(t, got.Completed)
	require.Equal(t, []domain.Action{domain.ActionRenamed, domain.ActionCompleted}, recorded)
}

func TestUpdateReopen(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, true)
	expectTx(d.tx)
	d.todos.EXPECT().Get(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Append(inTx(), action(domain.ActionReopened)).Return(nil).Once()

	got, err := svc.Update(context.Background(), todo.ID, ports.UpdateTodoInput{Completed: ptr(false)})

	require.NoError(t, err)
	require.False(t, got.Completed)
}

func TestUpdateWithoutChangeWritesNothing(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	expectTx(d.tx)
	d.todos.EXPECT().Get(inTx(), todo.ID).Return(todo, nil).Once()

	_, err := svc.Update(context.Background(), todo.ID, ports.UpdateTodoInput{Title: ptr("old"), Completed: ptr(false)})

	require.NoError(t, err)
}

func TestUpdateActivityFailureIsReturned(t *testing.T) {
	svc, d := newService(t)
	todo := existingTodo(t, false)
	expectTx(d.tx)
	d.todos.EXPECT().Get(inTx(), todo.ID).Return(todo, nil).Once()
	d.todos.EXPECT().Update(inTx(), todo).Return(nil).Once()
	d.activities.EXPECT().Append(inTx(), mock.Anything).Return(errBoom).Once()

	_, err := svc.Update(context.Background(), todo.ID, ports.UpdateTodoInput{Completed: ptr(true)})

	require.ErrorIs(t, err, errBoom)
}

func TestUpdateRejectsBadInputBeforeTx(t *testing.T) {
	svc, _ := newService(t)

	_, err := svc.Update(context.Background(), uuid.NewString(), ports.UpdateTodoInput{})
	require.ErrorIs(t, err, domain.ErrEmptyUpdate)

	_, err = svc.Update(context.Background(), "not-a-uuid", ports.UpdateTodoInput{Completed: ptr(true)})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateUnknownID(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().Get(inTx(), id).Return(nil, domain.ErrNotFound).Once()

	_, err := svc.Update(context.Background(), id, ports.UpdateTodoInput{Completed: ptr(true)})

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDelete(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().Delete(inTx(), id).Return(nil).Once()
	d.activities.EXPECT().Append(inTx(), action(domain.ActionDeleted)).Return(nil).Once()

	require.NoError(t, svc.Delete(context.Background(), id))
}

func TestDeleteUnknownIDRecordsNothing(t *testing.T) {
	svc, d := newService(t)
	id := uuid.NewString()
	expectTx(d.tx)
	d.todos.EXPECT().Delete(inTx(), id).Return(domain.ErrNotFound).Once()

	require.ErrorIs(t, svc.Delete(context.Background(), id), domain.ErrNotFound)
}

func TestMalformedIDNeverReachesRepositories(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	_, err := svc.Get(ctx, "not-a-uuid")
	require.ErrorIs(t, err, domain.ErrNotFound)

	require.ErrorIs(t, svc.Delete(ctx, "not-a-uuid"), domain.ErrNotFound)

	activities, err := svc.ListActivities(ctx, "not-a-uuid")
	require.NoError(t, err)
	require.Empty(t, activities)
}
