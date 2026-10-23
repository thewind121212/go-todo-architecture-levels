// Package handler is the HTTP layer: it parses requests, calls the service
// and writes responses. It holds no business rules.
package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level03/internal/service"
)

// Handler depends on the concrete service: there is only one implementation,
// and tests run it against fake repositories, so an interface would add
// nothing.
type Handler struct {
	todos *service.TodoService
	ping  func(context.Context) error
	log   zerolog.Logger
}

// NewRouter wires the routes. ping checks the database for GET /health.
func NewRouter(todos *service.TodoService, ping func(context.Context) error, log zerolog.Logger) *gin.Engine {
	h := &Handler{todos: todos, ping: ping, log: log}

	// gin.New instead of gin.Default: requestLogger replaces Gin's text
	// logger, so each request is logged once, as zerolog JSON.
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(log))
	r.GET("/health", h.health)
	r.POST("/todos", h.createTodo)
	r.GET("/todos", h.listTodos)
	r.GET("/todos/:id", h.getTodo)
	r.PATCH("/todos/:id", h.updateTodo)
	r.DELETE("/todos/:id", h.deleteTodo)
	r.GET("/todos/:id/activities", h.listActivities)
	return r
}

func requestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}

func (h *Handler) health(c *gin.Context) {
	if err := h.ping(c.Request.Context()); err != nil {
		h.writeError(c, fmt.Errorf("ping: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
