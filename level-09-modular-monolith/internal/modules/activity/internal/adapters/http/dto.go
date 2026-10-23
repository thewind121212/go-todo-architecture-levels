package http

import (
	"time"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

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

func toActivityResponse(a *application.Activity) activityResponse {
	return activityResponse{
		ID:        a.ID,
		TodoID:    a.TodoID,
		Action:    a.Action,
		CreatedAt: a.CreatedAt,
	}
}
