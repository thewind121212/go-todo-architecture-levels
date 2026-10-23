package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Request errors that belong to HTTP, not to the domain.
var (
	ErrInvalidJSON   = errors.New("invalid JSON body")
	ErrInvalidLimit  = errors.New("limit must be an integer from 1 to 100")
	ErrInvalidOffset = errors.New("offset must be an integer >= 0")
)

const (
	defaultLimit = 20
	maxLimit     = 100
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
		writeError(c, err)
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
	todo, err := h.store.Create(c.Request.Context(), req.Title)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, todo)
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
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	todo, err := h.store.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, todo)
}

func (h handler) updateTodo(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	var p todoPatch
	if err := c.ShouldBindJSON(&p); err != nil {
		writeError(c, ErrInvalidJSON)
		return
	}
	todo, err := h.store.Update(c.Request.Context(), id, p)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, todo)
}

func (h handler) deleteTodo(c *gin.Context) {
	id, err := parseID(c)
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
	id, err := parseID(c)
	if err != nil {
		writeError(c, err)
		return
	}
	activities, err := h.store.Activities(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": activities})
}

// parseID treats a malformed id like an unknown one. Checking here also keeps
// Postgres from failing with a 500 when it casts the string to uuid.
func parseID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", ErrNotFound
	}
	return id.String(), nil
}

func parsePage(c *gin.Context) (limit, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, 0, ErrInvalidLimit
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, ErrInvalidOffset
	}
	return limit, offset, nil
}

// writeError is the one place where errors become HTTP status codes.
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidTitle),
		errors.Is(err, ErrEmptyPatch),
		errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrInvalidLimit),
		errors.Is(err, ErrInvalidOffset):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
