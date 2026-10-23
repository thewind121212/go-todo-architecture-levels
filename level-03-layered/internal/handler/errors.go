package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"example.com/todo/level03/internal/model"
)

// HTTP-layer errors: they describe a malformed request, not a business rule,
// so they live here instead of in model.
var (
	errInvalidLimit  = errors.New("limit must be between 1 and 100")
	errInvalidOffset = errors.New("offset must be 0 or more")
	errInvalidJSON   = errors.New("invalid JSON body")
)

// writeError is the one place that turns an error into an HTTP status.
// Unknown errors are logged with their full chain and hidden from the client.
func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": model.ErrNotFound.Error()})
	case errors.Is(err, errInvalidJSON),
		errors.Is(err, model.ErrInvalidTitle),
		errors.Is(err, model.ErrEmptyPatch),
		errors.Is(err, errInvalidLimit),
		errors.Is(err, errInvalidOffset):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		h.log.Error().Err(err).Str("method", c.Request.Method).Str("path", c.Request.URL.Path).Msg("internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
