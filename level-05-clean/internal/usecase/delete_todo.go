package usecase

import (
	"context"

	"example.com/todo/level05/internal/entity"
)

type DeleteTodoInput struct {
	ID string
}

// DeleteTodo hard-deletes a todo and records its "deleted" activity. The activities are kept.
type DeleteTodo struct {
	todos      TodoRepository
	activities ActivityRepository
	tx         TxManager
}

func NewDeleteTodo(todos TodoRepository, activities ActivityRepository, tx TxManager) *DeleteTodo {
	return &DeleteTodo{todos: todos, activities: activities, tx: tx}
}

// Execute returns only an error: a deleted todo has nothing left to show.
func (uc *DeleteTodo) Execute(ctx context.Context, in DeleteTodoInput) error {
	deleted, err := entity.NewActivity(in.ID, entity.ActionDeleted, nowUTC())
	if err != nil {
		return err
	}
	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.todos.Delete(ctx, in.ID); err != nil {
			return err
		}
		return uc.activities.Create(ctx, deleted)
	})
}
