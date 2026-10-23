package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"example.com/todo/level03/internal/model"
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
	limit, offset, err := parsePage(c)
	if err != nil {
		h.writeError(c, err)
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
	id, err := todoID(c)
	if err != nil {
		h.writeError(c, err)
		return
	}
	todo, err := h.todos.Get(c.Request.Context(), id)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *Handler) updateTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		h.writeError(c, err)
		return
	}
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	todo, err := h.todos.Update(c.Request.Context(), id, model.Patch{Title: req.Title, Completed: req.Completed})
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *Handler) deleteTodo(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if err := h.todos.Delete(c.Request.Context(), id); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listActivities(c *gin.Context) {
	id, err := todoID(c)
	if err != nil {
		h.writeError(c, err)
		return
	}
	activities, err := h.todos.ListActivities(c.Request.Context(), id)
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

// todoID reads the :id path parameter. A malformed id can't match any todo,
// so it is reported as not found rather than as a bad request.
func todoID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", model.ErrNotFound
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
