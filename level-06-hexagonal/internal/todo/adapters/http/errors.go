package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level06/internal/todo/domain"
)

// Request-shape errors that only the HTTP adapter can detect.
var (
	errInvalidJSON   = errors.New("invalid JSON body")
	errInvalidLimit  = errors.New("limit must be between 1 and 100")
	errInvalidOffset = errors.New("offset must be 0 or more")
)

// writeError is the one place that maps errors to HTTP status codes.
func writeError(c *gin.Context, log zerolog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: domain.ErrNotFound.Error()})
	case errors.Is(err, domain.ErrInvalidTitle),
		errors.Is(err, domain.ErrEmptyPatch),
		errors.Is(err, errInvalidJSON),
		errors.Is(err, errInvalidLimit),
		errors.Is(err, errInvalidOffset):
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
	default:
		log.Error().Err(err).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("request failed")
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}
