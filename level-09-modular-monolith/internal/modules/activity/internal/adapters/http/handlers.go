// Package http is the activity module's driving adapter: it serves the activity log.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// handler serves the activity endpoint.
type handler struct {
	queries application.ActivityQueries
	log     zerolog.Logger
}

func (h *handler) listActivities(c *gin.Context) {
	activities, err := h.queries.ListActivities(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	items := make([]activityResponse, 0, len(activities))
	for _, a := range activities {
		items = append(items, toActivityResponse(a))
	}
	c.JSON(http.StatusOK, listActivitiesResponse{Items: items})
}
