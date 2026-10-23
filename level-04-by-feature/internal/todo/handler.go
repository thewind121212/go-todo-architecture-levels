package todo

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level04/internal/platform/httpx"
)

const defaultLimit = 20

var errInvalidJSON = errors.New("invalid JSON body")

type Handler struct {
	svc *Service
	log zerolog.Logger
}

func NewHandler(svc *Service, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(r gin.IRouter) {
	r.POST("/todos", h.create)
	r.GET("/todos", h.list)
	r.GET("/todos/:id", h.get)
	r.PATCH("/todos/:id", h.update)
	r.DELETE("/todos/:id", h.delete)
}

type createRequest struct {
	Title string `json:"title"`
}

type updateRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

type listResponse struct {
	Items  []Todo `json:"items"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), req.Title)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *Handler) list(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil {
		h.writeError(c, ErrInvalidLimit)
		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		h.writeError(c, ErrInvalidOffset)
		return
	}
	todos, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, listResponse{Items: todos, Limit: limit, Offset: offset})
}

func (h *Handler) get(c *gin.Context) {
	t, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeError(c, errInvalidJSON)
		return
	}
	t, err := h.svc.Update(c.Request.Context(), c.Param("id"), Patch{Title: req.Title, Completed: req.Completed})
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeError is the one place where this feature maps errors to status codes.
func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
	case errors.Is(err, ErrInvalidTitle),
		errors.Is(err, ErrEmptyUpdate),
		errors.Is(err, ErrInvalidLimit),
		errors.Is(err, ErrInvalidOffset),
		errors.Is(err, errInvalidJSON):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		httpx.InternalError(c, h.log, err)
	}
}
