package http

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level08/internal/todo/ports"
)

// RegisterRoutes mounts the todo endpoints. It depends on the two driving ports
// only: writes go to ports.TodoCommands, reads to ports.TodoQueries.
func RegisterRoutes(router gin.IRouter, commands ports.TodoCommands, queries ports.TodoQueries, log zerolog.Logger) {
	h := &handler{commands: commands, queries: queries, log: log}
	todos := router.Group("/todos")
	todos.POST("", h.create)
	todos.GET("", h.list)
	todos.GET("/:id", h.get)
	todos.PATCH("/:id", h.update)
	todos.DELETE("/:id", h.delete)
	todos.GET("/:id/activities", h.listActivities)
}
