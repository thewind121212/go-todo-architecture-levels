package usecase

import (
	"context"
	"time"

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
	// Validate the input before touching the database, so a bad patch is a 400 even for an unknown id.
	if in.Title == nil && in.Completed == nil {
		return TodoOutput{}, entity.ErrEmptyPatch
	}
	if in.Title != nil {
		if _, err := entity.ValidateTitle(*in.Title); err != nil {
			return TodoOutput{}, err
		}
	}

	now := nowUTC()
	var todo entity.Todo
	// Inside the transaction only the diff happens: read the current todo (locked), work out what
	// changed, and write the todo with one activity per change.
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		todo, err = uc.todos.GetForUpdate(ctx, in.ID)
		if err != nil {
			return err
		}

		actions, err := applyChanges(&todo, in, now)
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
			activity, err := entity.NewActivity(todo.ID, action, now)
			if err != nil {
				return err
			}
			if err := uc.activities.Create(ctx, activity); err != nil {
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

// applyChanges calls the entity's methods for the requested fields and returns one action per
// real change, renamed first.
func applyChanges(todo *entity.Todo, in UpdateTodoInput, now time.Time) ([]entity.Action, error) {
	var actions []entity.Action
	if in.Title != nil {
		renamed, err := todo.Rename(*in.Title, now)
		if err != nil {
			return nil, err
		}
		if renamed {
			actions = append(actions, entity.ActionRenamed)
		}
	}
	if in.Completed != nil {
		if action, changed := todo.SetCompleted(*in.Completed, now); changed {
			actions = append(actions, action)
		}
	}
	return actions, nil
}
