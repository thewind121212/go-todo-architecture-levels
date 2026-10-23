// Package http is a driving adapter: it turns HTTP requests into calls on
// the ports.TodoService driving port. It knows nothing about databases.
package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

var (
	errInvalidJSON   = errors.New("invalid JSON body")
	errInvalidLimit  = errors.New("limit must be an integer between 1 and 100")
	errInvalidOffset = errors.New("offset must be an integer >= 0")
)

// handler serves the todo endpoints.
type handler struct {
	svc ports.TodoService
	log zerolog.Logger
}

func (h *handler) create(c *gin.Context) {
	var req createTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	todo, err := h.svc.Create(c.Request.Context(), req.Title)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTodoResponse(todo))
}

func (h *handler) list(c *gin.Context) {
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil || limit < 1 || limit > maxLimit {
		h.writeError(c, errInvalidLimit)
		return
	}
	offset, err := queryInt(c, "offset", 0)
	if err != nil || offset < 0 {
		h.writeError(c, errInvalidOffset)
		return
	}
	todos, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.writeError(c, err)
		return
	}
	items := make([]todoResponse, 0, len(todos))
	for _, t := range todos {
		items = append(items, toTodoResponse(t))
	}
	c.JSON(http.StatusOK, listTodosResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *handler) get(c *gin.Context) {
	todo, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *handler) update(c *gin.Context) {
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	in := ports.UpdateTodoInput{Title: req.Title, Completed: req.Completed}
	todo, err := h.svc.Update(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *handler) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handler) listActivities(c *gin.Context) {
	activities, err := h.svc.ListActivities(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	items := make([]activityResponse, 0, len(activities))
	for _, a := range activities {
		items = append(items, toActivityResponse(a))
	}
	c.JSON(http.StatusOK, listActivitiesResponse{Items: items})
}

// writeError is the one place where errors become HTTP status codes.
func (h *handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: domain.ErrNotFound.Error()})
	case errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrEmptyUpdate),
		errors.Is(err, errInvalidJSON),
		errors.Is(err, errInvalidLimit),
		errors.Is(err, errInvalidOffset):
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		h.log.Error().Err(err).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("request failed")
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}

// queryInt reads an integer query parameter, or def when it is absent.
func queryInt(c *gin.Context, key string, def int) (int, error) {
	raw, ok := c.GetQuery(key)
	if !ok {
		return def, nil
	}
	return strconv.Atoi(raw)
}
