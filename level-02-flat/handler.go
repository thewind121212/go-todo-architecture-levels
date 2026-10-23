// handler.go: Gin routes and handlers, and writeError, the one error-to-status mapping.

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handler struct {
	store store
}

func newRouter(s store) *gin.Engine {
	h := handler{store: s}
	r := gin.Default()
	r.GET("/health", h.health)
	r.POST("/todos", h.createTodo)
	r.GET("/todos", h.listTodos)
	r.GET("/todos/:id", h.getTodo)
	r.PATCH("/todos/:id", h.updateTodo)
	r.DELETE("/todos/:id", h.deleteTodo)
	r.GET("/todos/:id/activities", h.listActivities)
	return r
}

func (h handler) health(c *gin.Context) {
	if err := h.store.Ping(c.Request.Context()); err != nil {
		writeError(c, fmt.Errorf("ping: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h handler) createTodo(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, ErrInvalidJSON)
		return
	}
	t, err := newTodo(req.Title)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.store.Create(c.Request.Context(), t); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h handler) listTodos(c *gin.Context) {
	limit, offset, err := parsePage(c)
	if err != nil {
		writeError(c, err)
		return
	}
	todos, err := h.store.List(c.Request.Context(), limit, offset)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": todos, "limit": limit, "offset": offset})
}

func (h handler) getTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	t, err := h.store.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h handler) updateTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var p Patch
	if err := c.ShouldBindJSON(&p); err != nil {
		writeError(c, ErrInvalidJSON)
		return
	}
	p, err = validatePatch(p)
	if err != nil {
		writeError(c, err)
		return
	}
	t, err := h.store.Update(c.Request.Context(), id, p)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h handler) deleteTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h handler) listActivities(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	activities, err := h.store.ListActivities(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": activities})
}

// todoID reads the :id path parameter. A malformed id can't match any todo,
// so it is reported as not found rather than as a bad request.
func todoID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", ErrNotFound
	}
	return id.String(), nil
}

func parsePage(c *gin.Context) (limit, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, 0, ErrInvalidLimit
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, ErrInvalidOffset
	}
	return limit, offset, nil
}

// writeError is the one place that turns an error into an HTTP status.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrInvalidTitle),
		errors.Is(err, ErrEmptyPatch),
		errors.Is(err, ErrInvalidLimit),
		errors.Is(err, ErrInvalidOffset):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
