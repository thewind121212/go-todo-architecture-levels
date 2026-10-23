// Package health serves the health endpoints.
package health

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// CheckFunc pings the database. Each storage choice provides its own.
type CheckFunc func(ctx context.Context) error

// RegisterRoutes mounts:
//   - /health and /health/ready: 200 only when the database answers
//   - /health/live: 200 while the process runs, without touching the database
func RegisterRoutes(router gin.IRouter, check CheckFunc, log zerolog.Logger) {
	ready := func(c *gin.Context) {
		if err := check(c.Request.Context()); err != nil {
			log.Error().Err(err).Msg("health check failed")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	router.GET("/health", ready)
	router.GET("/health/ready", ready)
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
