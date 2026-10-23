package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// writeError is the one place in this module that maps errors to HTTP status codes.
// Each module has its own: it knows only its own errors.
func writeError(c *gin.Context, log zerolog.Logger, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: application.ErrNotFound.Error()})
	default:
		log.Error().Err(err).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("request failed")
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}
