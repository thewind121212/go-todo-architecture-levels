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
	limit, offset, err := parsePage(c)
	if err != nil {
		writeError(c, h.log, err)
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
	id, err := todoID(c)
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
	id, err := todoID(c)
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
	id, err := todoID(c)
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
	id, err := todoID(c)
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

// todoID reads the :id path parameter. A malformed id can't match any todo,
// so it is reported as not found rather than as a bad request.
func todoID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", domain.ErrNotFound
	}
	return id.String(), nil
}

func parsePage(c *gin.Context) (limit, offset int, err error) {
	limit, err = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, 0, errInvalidLimit
	}
	offset, err = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return 0, 0, errInvalidOffset
	}
	return limit, offset, nil
}
