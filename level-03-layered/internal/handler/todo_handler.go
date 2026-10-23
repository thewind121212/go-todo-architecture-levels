package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"example.com/todo/level03/internal/model"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

func (h *Handler) createTodo(c *gin.Context) {
	var req createTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	todo, err := h.todos.Create(c.Request.Context(), req.Title)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTodoResponse(todo))
}

func (h *Handler) listTodos(c *gin.Context) {
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
	todos, err := h.todos.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.writeError(c, err)
		return
	}
	items := make([]todoResponse, 0, len(todos))
	for _, todo := range todos {
		items = append(items, toTodoResponse(todo))
	}
	c.JSON(http.StatusOK, listTodosResponse{Items: items, Limit: limit, Offset: offset})
}

func (h *Handler) getTodo(c *gin.Context) {
	todo, err := h.todos.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *Handler) updateTodo(c *gin.Context) {
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	patch := model.Patch{Title: req.Title, Completed: req.Completed}
	todo, err := h.todos.Update(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *Handler) deleteTodo(c *gin.Context) {
	if err := h.todos.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listActivities(c *gin.Context) {
	activities, err := h.todos.ListActivities(c.Request.Context(), c.Param("id"))
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

// queryInt reads an integer query parameter, or returns def when it is absent.
func queryInt(c *gin.Context, key string, def int) (int, error) {
	s, ok := c.GetQuery(key)
	if !ok {
		return def, nil
	}
	return strconv.Atoi(s)
}
