package usecase

import (
	"context"

	"example.com/todo/level05/internal/entity"
)

// UpdateTodoInput uses pointers because a nil field means "leave it unchanged".
type UpdateTodoInput struct {
	ID        string
	Title     *string
	Completed *bool
}

// UpdateTodo renames and/or completes a todo, recording one activity per real change.
type UpdateTodo struct {
	todos      TodoRepository
	activities ActivityRepository
	tx         TxManager
}

func NewUpdateTodo(todos TodoRepository, activities ActivityRepository, tx TxManager) *UpdateTodo {
	return &UpdateTodo{todos: todos, activities: activities, tx: tx}
}

func (uc *UpdateTodo) Execute(ctx context.Context, in UpdateTodoInput) (TodoOutput, error) {
	if in.Title == nil && in.Completed == nil {
		return TodoOutput{}, ErrEmptyUpdate
	}

	var todo entity.Todo
	// The read is inside the transaction too, because the changes are computed from it.
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = uc.todos.FindByID(ctx, in.ID)
		if err != nil {
			return err
		}

		actions, err := applyChanges(&todo, in)
		if err != nil {
			return err
		}
		if len(actions) == 0 {
			return nil // nothing changed: no write, no activity
		}

		if err := uc.todos.Update(ctx, todo); err != nil {
			return err
		}
		for _, action := range actions {
			if err := uc.activities.Create(ctx, entity.NewActivity(todo.ID, action)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return TodoOutput{}, err
	}
	return newTodoOutput(todo), nil
}

// applyChanges applies the requested fields and returns one action per real change, in order.
func applyChanges(todo *entity.Todo, in UpdateTodoInput) ([]entity.Action, error) {
	var actions []entity.Action
	if in.Title != nil {
		renamed, err := todo.Rename(*in.Title)
		if err != nil {
			return nil, err
		}
		if renamed {
			actions = append(actions, entity.ActionRenamed)
		}
	}
	if in.Completed != nil {
		if action, changed := todo.SetCompleted(*in.Completed); changed {
			actions = append(actions, action)
		}
	}
	return actions, nil
}
