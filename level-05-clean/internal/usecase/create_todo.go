package usecase

import (
	"context"

	"example.com/todo/level05/internal/entity"
)

type CreateTodoInput struct {
	Title string
}

// CreateTodo creates a todo and records its "created" activity.
type CreateTodo struct {
	todos      TodoRepository
	activities ActivityRepository
	tx         TxManager
}

func NewCreateTodo(todos TodoRepository, activities ActivityRepository, tx TxManager) *CreateTodo {
	return &CreateTodo{todos: todos, activities: activities, tx: tx}
}

func (uc *CreateTodo) Execute(ctx context.Context, in CreateTodoInput) (TodoOutput, error) {
	todo, err := entity.NewTodo(in.Title)
	if err != nil {
		return TodoOutput{}, err
	}

	// The interactor owns the transaction: the todo and its activity are saved together or not at all.
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.todos.Create(ctx, todo); err != nil {
			return err
		}
		return uc.activities.Create(ctx, entity.NewActivity(todo.ID, entity.ActionCreated))
	})
	if err != nil {
		return TodoOutput{}, err
	}
	return newTodoOutput(todo), nil
}
