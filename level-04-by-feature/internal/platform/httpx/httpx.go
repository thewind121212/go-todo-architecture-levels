// Package httpx holds the HTTP helpers that both features need.
package httpx

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// RequestLogger logs each request once, as zerolog JSON.
func RequestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	}
}

// InternalError logs the real error and sends the client a generic message,
// so database details never leak into responses.
func InternalError(c *gin.Context, log zerolog.Logger, err error) {
	log.Error().Err(err).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Msg("internal error")
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
