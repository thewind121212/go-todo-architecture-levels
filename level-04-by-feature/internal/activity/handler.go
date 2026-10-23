package activity

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"example.com/todo/level04/internal/platform/httpx"
)

// errNotFound repeats todo.ErrNotFound's message: the :id in the path names a
// todo, but this feature must not import the todo package.
var errNotFound = errors.New("todo not found")

type activityResponse struct {
	ID        string    `json:"id"`
	TodoID    string    `json:"todo_id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

type listActivitiesResponse struct {
	Items []activityResponse `json:"items"`
}

func toActivityResponse(a Activity) activityResponse {
	return activityResponse{ID: a.ID, TodoID: a.TodoID, Action: a.Action, CreatedAt: a.CreatedAt}
}

type Handler struct {
	activities *Service
	log        zerolog.Logger
}

func NewHandler(activities *Service, log zerolog.Logger) *Handler {
	return &Handler{activities: activities, log: log}
}

// Register adds the activity route. It lives under /todos because that is the
// public API shape, but the code belongs to this feature.
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/todos/:id/activities", h.listActivities)
}

func (h *Handler) listActivities(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.writeError(c, errNotFound)
		return
	}
	activities, err := h.activities.List(c.Request.Context(), id.String())
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

// writeError is the one place where this feature's errors become HTTP status
// codes. Unknown errors are logged and hidden from the client.
func (h *Handler) writeError(c *gin.Context, err error) {
	if errors.Is(err, errNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": errNotFound.Error()})
		return
	}
	httpx.InternalError(c, h.log, err)
}
