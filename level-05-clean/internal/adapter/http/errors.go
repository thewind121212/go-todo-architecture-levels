package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

// Request-shape errors that only the web adapter can detect.
var (
	errInvalidJSON  = errors.New("invalid JSON body")
	errInvalidQuery = errors.New("limit and offset must be integers")
)

// writeError is the one place that maps errors to HTTP status codes.
func writeError(c *gin.Context, log zerolog.Logger, err error) {
	switch {
	case errors.Is(err, entity.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": entity.ErrNotFound.Error()})
	case errors.Is(err, entity.ErrInvalidTitle),
		errors.Is(err, usecase.ErrInvalidPagination),
		errors.Is(err, usecase.ErrEmptyUpdate),
		errors.Is(err, errInvalidJSON),
		errors.Is(err, errInvalidQuery):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Error().Err(err).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
