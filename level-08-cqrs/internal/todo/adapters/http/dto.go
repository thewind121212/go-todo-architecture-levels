package http

import (
	"time"

	"example.com/todo/level08/internal/todo/application/query"
	"example.com/todo/level08/internal/todo/domain"
)

type createTodoRequest struct {
	Title string `json:"title"`
}

// updateTodoRequest uses pointers so "absent" and "zero value" differ.
type updateTodoRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

type todoResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listTodosResponse struct {
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

type listActivitiesResponse struct {
	Items []activityResponse `json:"items"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// toTodoResponse answers a command with the state of the aggregate it just saved.
func toTodoResponse(t *domain.Todo) todoResponse {
	return todoResponse{
		ID:        t.ID().String(),
		Title:     t.Title().String(),
		Completed: t.Completed(),
		CreatedAt: t.CreatedAt(),
		UpdatedAt: t.UpdatedAt(),
	}
}

// toTodoResponseFromView answers a query with a view. Both give the same JSON.
func toTodoResponseFromView(v *query.TodoView) todoResponse {
	return todoResponse{
		ID:        v.ID,
		Title:     v.Title,
		Completed: v.Completed,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

func toActivityResponse(a *query.ActivityView) activityResponse {
	return activityResponse{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    a.Action,
		CreatedAt: a.CreatedAt,
	}
}
