package http

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// RegisterRoutes mounts the activity endpoint. Its path sits under /todos because the API
// put it there; a module owns its routes, not a URL prefix. The parameter must be called
// :id, like the todo module's, because Gin shares one path tree.
func RegisterRoutes(router gin.IRouter, queries application.ActivityQueries, log zerolog.Logger) {
	h := &handler{queries: queries, log: log}
	router.GET("/todos/:id/activities", h.listActivities)
}
