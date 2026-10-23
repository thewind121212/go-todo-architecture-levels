package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"example.com/todo/level08/internal/infrastructure/health"
)

func get(t *testing.T, check health.CheckFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	health.RegisterRoutes(router, check, zerolog.Nop())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	up := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("no db") }

	for _, path := range []string{"/health", "/health/ready"} {
		rec := get(t, up, path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.JSONEq(t, `{"status":"ok"}`, rec.Body.String(), path)

		rec = get(t, down, path)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, path)
		require.JSONEq(t, `{"error":"database unavailable"}`, rec.Body.String(), path)
	}

	rec := get(t, down, "/health/live")
	require.Equal(t, http.StatusOK, rec.Code, "liveness ignores the database")
}
