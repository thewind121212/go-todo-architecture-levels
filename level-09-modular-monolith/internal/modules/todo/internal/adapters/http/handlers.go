// Package http is a driving adapter: it turns HTTP requests into calls on the driving
// ports. POST, PATCH and DELETE are commands; GETs are queries. It knows nothing about databases.
package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level09/internal/modules/todo/internal/ports"
)

// handler serves the todo endpoints.
type handler struct {
	commands ports.TodoCommands
	queries  ports.TodoQueries
	log      zerolog.Logger
}

func (h *handler) create(c *gin.Context) {
	var req createTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, h.log, errInvalidJSON)
		return
	}
	todo, err := h.commands.CreateTodo(c.Request.Context(), req.Title)
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
	views, err := h.queries.ListTodos(c.Request.Context(), limit, offset)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	items := make([]todoResponse, 0, len(views))
	for _, v := range views {
		items = append(items, toTodoResponseFromView(v))
	}
	c.JSON(http.StatusOK, listTodosResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *handler) get(c *gin.Context) {
	view, err := h.queries.GetTodo(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponseFromView(view))
}

func (h *handler) update(c *gin.Context) {
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, h.log, errInvalidJSON)
		return
	}
	in := ports.UpdateTodoInput{ID: c.Param("id"), Title: req.Title, Completed: req.Completed}
	todo, err := h.commands.UpdateTodo(c.Request.Context(), in)
	if err != nil {
		writeError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *handler) delete(c *gin.Context) {
	if err := h.commands.DeleteTodo(c.Request.Context(), c.Param("id")); err != nil {
		writeError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
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
