package usecase

import "context"

type GetTodoInput struct {
	ID string
}

// GetTodo returns one todo.
type GetTodo struct {
	todos TodoRepository
}

func NewGetTodo(todos TodoRepository) *GetTodo {
	return &GetTodo{todos: todos}
}

func (uc *GetTodo) Execute(ctx context.Context, in GetTodoInput) (TodoOutput, error) {
	todo, err := uc.todos.FindByID(ctx, in.ID)
	if err != nil {
		return TodoOutput{}, err
	}
	return newTodoOutput(todo), nil
}
