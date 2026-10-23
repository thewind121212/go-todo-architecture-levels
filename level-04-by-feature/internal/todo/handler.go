package todo

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"example.com/todo/level04/internal/platform/httpx"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Request errors found by the handler itself, before the service is called.
var (
	errInvalidJSON   = errors.New("invalid JSON body")
	errInvalidLimit  = errors.New("limit must be between 1 and 100")
	errInvalidOffset = errors.New("offset must be 0 or more")
)

// Request and response shapes live here, so JSON names never leak into Todo.

type createTodoRequest struct {
	Title string `json:"title"`
}

// Pointers tell "field absent" (nil) apart from a zero value like false.
type updateTodoRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

type todoResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listTodosResponse struct {
	Items  []todoResponse `json:"items"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func toTodoResponse(t Todo) todoResponse {
	return todoResponse{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

// Handler depends on the concrete *Service: there is only one implementation,
// and tests run it against fakes, so an interface would add nothing.
type Handler struct {
	todos *Service
	log   zerolog.Logger
}

func NewHandler(todos *Service, log zerolog.Logger) *Handler {
	return &Handler{todos: todos, log: log}
}

func (h *Handler) Register(r gin.IRouter) {
	r.POST("/todos", h.createTodo)
	r.GET("/todos", h.listTodos)
	r.GET("/todos/:id", h.getTodo)
	r.PATCH("/todos/:id", h.updateTodo)
	r.DELETE("/todos/:id", h.deleteTodo)
}

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
	id, err := parseID(c)
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
	id, err := parseID(c)
	if err != nil {
		h.writeError(c, err)
		return
	}
	var req updateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	patch := Patch{Title: req.Title, Completed: req.Completed}
	todo, err := h.todos.Update(c.Request.Context(), id, patch)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTodoResponse(todo))
}

func (h *Handler) deleteTodo(c *gin.Context) {
	id, err := parseID(c)
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

// parseID returns the canonical form of the :id path parameter. A malformed
// id can never match a todo, so it is ErrNotFound.
func parseID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", ErrNotFound
	}
	return id.String(), nil
}

// queryInt reads an integer query parameter, or returns def when it is absent.
func queryInt(c *gin.Context, key string, def int) (int, error) {
	s, ok := c.GetQuery(key)
	if !ok {
		return def, nil
	}
	return strconv.Atoi(s)
}

// writeError is the one place where this feature's errors become HTTP status
// codes. Unknown errors are logged and hidden from the client.
func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
	case errors.Is(err, ErrInvalidTitle),
		errors.Is(err, ErrEmptyPatch),
		errors.Is(err, errInvalidJSON),
		errors.Is(err, errInvalidLimit),
		errors.Is(err, errInvalidOffset):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		httpx.InternalError(c, h.log, err)
	}
}
