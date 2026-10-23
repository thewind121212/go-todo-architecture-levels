package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// NewRouter wires the routes. ping checks that the database is reachable.
func NewRouter(todos *TodoController, activities *ActivityController, ping func(context.Context) error, log zerolog.Logger) *gin.Engine {
	// gin.New instead of gin.Default: requestLogger replaces Gin's text
	// logger, so each request is logged once, as zerolog JSON.
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(log))
	r.GET("/health", health(ping, log))
	r.POST("/todos", todos.Create)
	r.GET("/todos", todos.List)
	r.GET("/todos/:id", todos.Get)
	r.PATCH("/todos/:id", todos.Update)
	r.DELETE("/todos/:id", todos.Delete)
	r.GET("/todos/:id/activities", activities.List)
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

func health(ping func(context.Context) error, log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ping(c.Request.Context()); err != nil {
			writeError(c, log, fmt.Errorf("ping: %w", err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
