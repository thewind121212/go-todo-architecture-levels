// Package httpx holds the HTTP helpers that both features need.
package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// InternalError logs the real error and sends the client a generic message,
// so database details never leak into responses.
func InternalError(c *gin.Context, log zerolog.Logger, err error) {
	log.Error().Err(err).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Msg("request failed")
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
