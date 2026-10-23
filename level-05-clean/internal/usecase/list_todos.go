package usecase

import "context"

const maxLimit = 100

type ListTodosInput struct {
	Limit  int
	Offset int
}

type ListTodosOutput struct {
	Items  []TodoOutput
	Limit  int
	Offset int
}

// ListTodos returns one page of todos, newest first.
type ListTodos struct {
	todos TodoRepository
}

func NewListTodos(todos TodoRepository) *ListTodos {
	return &ListTodos{todos: todos}
}

func (uc *ListTodos) Execute(ctx context.Context, in ListTodosInput) (ListTodosOutput, error) {
	if in.Limit < 1 || in.Limit > maxLimit || in.Offset < 0 {
		return ListTodosOutput{}, ErrInvalidPagination
	}
	todos, err := uc.todos.List(ctx, in.Limit, in.Offset)
	if err != nil {
		return ListTodosOutput{}, err
	}
	items := make([]TodoOutput, 0, len(todos))
	for _, t := range todos {
		items = append(items, newTodoOutput(t))
	}
	return ListTodosOutput{Items: items, Limit: in.Limit, Offset: in.Offset}, nil
}
