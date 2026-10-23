package handler

import (
	"time"

	"example.com/todo/level03/internal/model"
)

// Request and response shapes live here, so JSON names never leak into the model.

type createTodoRequest struct {
	Title string `json:"title"`
}

// Pointers tell "field absent" (nil) apart from a zero value like false.
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

func toTodoResponse(t model.Todo) todoResponse {
	return todoResponse{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func toActivityResponse(a model.Activity) activityResponse {
	return activityResponse{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
}
