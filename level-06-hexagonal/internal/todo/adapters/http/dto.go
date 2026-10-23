package http

import (
	"time"

	"example.com/todo/level06/internal/todo/domain"
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

func toTodoResponse(t *domain.Todo) todoResponse {
	return todoResponse{
		ID:        t.ID,
		Title:     t.Title,
		Completed: t.Completed,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toActivityResponse(a *domain.Activity) activityResponse {
	return activityResponse{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    string(a.Action),
		CreatedAt: a.CreatedAt,
	}
}
