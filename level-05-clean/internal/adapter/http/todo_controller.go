// Package http is the web adapter: Gin controllers turn requests into use case inputs, and presenters
// turn use case outputs into JSON.
package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level05/internal/usecase"
)

// The controller depends on these input ports, not on the concrete interactors. They are declared
// here, by the consumer, and each has one method, so a controller test can fake one with a func.
type (
	CreateTodoUseCase interface {
		Execute(ctx context.Context, in usecase.CreateTodoInput) (usecase.TodoOutput, error)
	}
	GetTodoUseCase interface {
		Execute(ctx context.Context, in usecase.GetTodoInput) (usecase.TodoOutput, error)
	}
	ListTodosUseCase interface {
		Execute(ctx context.Context, in usecase.ListTodosInput) (usecase.ListTodosOutput, error)
	}
	UpdateTodoUseCase interface {
		Execute(ctx context.Context, in usecase.UpdateTodoInput) (usecase.TodoOutput, error)
	}
	DeleteTodoUseCase interface {
		Execute(ctx context.Context, in usecase.DeleteTodoInput) error
	}
)

const defaultLimit = 20

type TodoController struct {
	create CreateTodoUseCase
	get    GetTodoUseCase
	list   ListTodosUseCase
	update UpdateTodoUseCase
	delete DeleteTodoUseCase
	log    zerolog.Logger
}

func NewTodoController(
	create CreateTodoUseCase,
	get GetTodoUseCase,
	list ListTodosUseCase,
	update UpdateTodoUseCase,
	del DeleteTodoUseCase,
	log zerolog.Logger,
) *TodoController {
	return &TodoController{create: create, get: get, list: list, update: update, delete: del, log: log}
}

type createTodoRequest struct {
	Title string `json:"title"`
}

// A nil field means the client did not send it.
type updateTodoRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

func (tc *TodoController) Create(c *gin.Context) {
	var req createTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, tc.log, errInvalidJSON)
		return
	}
	out, err := tc.create.Execute(c.Request.Context(), usecase.CreateTodoInput{Title: req.Title})
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	c.JSON(http.StatusCreated, presentTodo(out))
}

func (tc *TodoController) Get(c *gin.Context) {
	out, err := tc.get.Execute(c.Request.Context(), usecase.GetTodoInput{ID: c.Param("id")})
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	c.JSON(http.StatusOK, presentTodo(out))
}

func (tc *TodoController) List(c *gin.Context) {
	limit, err := queryInt(c, "limit", defaultLimit)
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	offset, err := queryInt(c, "offset", 0)
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	out, err := tc.list.Execute(c.Request.Context(), usecase.ListTodosInput{Limit: limit, Offset: offset})
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	c.JSON(http.StatusOK, presentTodoList(out))
}

func (tc *TodoController) Update(c *gin.Context) {
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, tc.log, errInvalidJSON)
		return
	}
	in := usecase.UpdateTodoInput{ID: c.Param("id"), Title: req.Title, Completed: req.Completed}
	out, err := tc.update.Execute(c.Request.Context(), in)
	if err != nil {
		writeError(c, tc.log, err)
		return
	}
	c.JSON(http.StatusOK, presentTodo(out))
}

func (tc *TodoController) Delete(c *gin.Context) {
	if err := tc.delete.Execute(c.Request.Context(), usecase.DeleteTodoInput{ID: c.Param("id")}); err != nil {
		writeError(c, tc.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// queryInt reads an optional integer query parameter. Range checks are the use case's job.
func queryInt(c *gin.Context, key string, fallback int) (int, error) {
	raw, ok := c.GetQuery(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errInvalidQuery
	}
	return n, nil
}
