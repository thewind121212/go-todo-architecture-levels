package activity

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level04/internal/platform/httpx"
)

type Handler struct {
	svc *Service
	log zerolog.Logger
}

func NewHandler(svc *Service, log zerolog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register adds the activity route. It lives under /todos because that is the
// public API shape, but the code belongs to this feature.
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/todos/:id/activities", h.list)
}

type listResponse struct {
	Items []Activity `json:"items"`
}

func (h *Handler) list(c *gin.Context) {
	activities, err := h.svc.List(c.Request.Context(), c.Param("id"))
	if err != nil {
		// Listing has no client errors, so every failure is a 500.
		httpx.InternalError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, listResponse{Items: activities})
}
