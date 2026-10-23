//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"example.com/todo/level06/internal/config"
)

// TestEndToEnd drives the Wire-generated Postgres graph through HTTP:
// router -> handler -> service -> repositories -> real database.
func TestEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("todo"),
		tcpostgres.WithUsername("todo"),
		tcpostgres.WithPassword("todo"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	router, cleanup, err := initPostgresApp(ctx, config.Config{PostgresDSN: dsn}, zerolog.Nop())
	require.NoError(t, err)
	t.Cleanup(cleanup)

	call := func(method, path, body string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		var out map[string]any
		if rec.Body.Len() > 0 {
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		}
		return rec.Code, out
	}

	status, todo := call(http.MethodPost, "/todos", `{"title":"e2e"}`)
	require.Equal(t, http.StatusCreated, status)
	path := "/todos/" + todo["id"].(string)

	status, todo = call(http.MethodPatch, path, `{"title":"e2e v2","completed":true}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "e2e v2", todo["title"])

	status, body := call(http.MethodGet, path+"/activities", "")
	require.Equal(t, http.StatusOK, status)
	var actions []string
	for _, item := range body["items"].([]any) {
		actions = append(actions, item.(map[string]any)["action"].(string))
	}
	require.Equal(t, []string{"created", "renamed", "completed"}, actions)

	// Input is validated before the database: a bad patch on an unknown id is 400, not 404.
	status, _ = call(http.MethodPatch, "/todos/"+uuid.NewString(), `{}`)
	require.Equal(t, http.StatusBadRequest, status)
}
