// Package http is a driving adapter: it turns HTTP requests into calls on
// the ports.TodoService driving port. It knows nothing about databases.
package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// handler serves the todo endpoints.
type handler struct {
	svc ports.TodoService
	log zerolog.Logger
}

func (h *handler) create(c *gin.Context) {
	var req createTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, h.log, errInvalidJSON)
		return
	}
	todo, err := h.svc.Create(c.Request.Context(), req.Title)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, toTodoResponse(todo))
}

func (h *handler) list(c *gin.Context) {
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil || limit < 1 || limit > maxLimit {
		writeError(c, h.log, errInvalidLimit)
		return
	}
	offset, err := queryInt(c, "offset", 0)
	if err != nil || offset < 0 {
		writeError(c, h.log, errInvalidOffset)
		return
	}
	todos, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	items := make([]todoResponse, 0, len(todos))
	for _, t := range todos {
		items = append(items, toTodoResponse(t))
	}
	c.JSON(http.StatusOK, listTodosResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *handler) get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	todo, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *handler) update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, h.log, errInvalidJSON)
		return
	}
	in := ports.UpdateTodoInput{ID: id, Title: req.Title, Completed: req.Completed}
	todo, err := h.svc.Update(c.Request.Context(), in)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *handler) delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handler) listActivities(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	activities, err := h.svc.ListActivities(c.Request.Context(), id)
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

// parseID returns the canonical form of the :id param. A malformed id cannot
// name any todo, so it is "not found", and the core never sees it.
func parseID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", domain.ErrNotFound
	}
	return id.String(), nil
}

// queryInt reads an optional integer query parameter, or fallback when it is absent.
func queryInt(c *gin.Context, key string, fallback int) (int, error) {
	raw, ok := c.GetQuery(key)
	if !ok {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}
