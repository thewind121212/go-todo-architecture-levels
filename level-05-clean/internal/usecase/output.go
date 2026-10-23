package usecase

import (
	"time"

	"example.com/todo/level05/internal/entity"
)

// TodoOutput is the data a use case hands back. Returning a plain struct instead of the entity keeps
// the outer layers from calling entity methods and bypassing the use cases.
type TodoOutput struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ActivityOutput is one activity as returned by a use case.
type ActivityOutput struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time
}

func newTodoOutput(t entity.Todo) TodoOutput {
	return TodoOutput{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func newActivityOutput(a entity.Activity) ActivityOutput {
	return ActivityOutput{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
}
