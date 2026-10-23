package http

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level07/internal/todo/ports"
)

// RegisterRoutes mounts the todo endpoints. It depends on the driving port
// only, so any ports.TodoService (the real service or a mock) can sit behind it.
func RegisterRoutes(router gin.IRouter, svc ports.TodoService, log zerolog.Logger) {
	h := &handler{svc: svc, log: log}
	todos := router.Group("/todos")
	todos.POST("", h.create)
	todos.GET("", h.list)
	todos.GET("/:id", h.get)
	todos.PATCH("/:id", h.update)
	todos.DELETE("/:id", h.delete)
	todos.GET("/:id/activities", h.listActivities)
}
