package http

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// NewRouter wires the routes. ping checks that the database is reachable.
func NewRouter(todos *TodoController, activities *ActivityController, ping func(context.Context) error, log zerolog.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", health(ping, log))
	r.POST("/todos", todos.Create)
	r.GET("/todos", todos.List)
	r.GET("/todos/:id", todos.Get)
	r.PATCH("/todos/:id", todos.Update)
	r.DELETE("/todos/:id", todos.Delete)
	r.GET("/todos/:id/activities", activities.List)
	return r
}

func health(ping func(context.Context) error, log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ping(c.Request.Context()); err != nil {
			writeError(c, log, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
