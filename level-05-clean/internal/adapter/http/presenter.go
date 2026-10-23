package http

import (
	"time"

	"example.com/todo/level05/internal/usecase"
)

// Presenters map use case outputs to the JSON shape of the API. JSON tags live here, in the
// adapter, and nowhere in the business core.

type todoResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type todoListResponse struct {
	Items  []todoResponse `json:"items"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type activityResponse struct {
	ID        string    `json:"id"`
	TodoID    string    `json:"todo_id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

type activityListResponse struct {
	Items []activityResponse `json:"items"`
}

func presentTodo(out usecase.TodoOutput) todoResponse {
	return todoResponse{
		ID:        out.ID,
		Title:     out.Title,
		Completed: out.Completed,
		CreatedAt: out.CreatedAt,
		UpdatedAt: out.UpdatedAt,
	}
}

// presentTodoList always returns a non-nil slice, so the JSON is [] and never null.
func presentTodoList(out usecase.ListTodosOutput) todoListResponse {
	items := make([]todoResponse, 0, len(out.Items))
	for _, t := range out.Items {
		items = append(items, presentTodo(t))
	}
	return todoListResponse{Items: items, Limit: out.Limit, Offset: out.Offset}
}

func presentActivityList(out usecase.ListActivitiesOutput) activityListResponse {
	items := make([]activityResponse, 0, len(out.Items))
	for _, a := range out.Items {
		items = append(items, activityResponse{ID: a.ID, TodoID: a.TodoID, Action: a.Action, CreatedAt: a.CreatedAt})
	}
	return activityListResponse{Items: items}
}
