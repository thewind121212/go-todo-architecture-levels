package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	activityhttp "example.com/todo/level09/internal/modules/activity/internal/adapters/http"
	"example.com/todo/level09/internal/modules/activity/internal/application"
	"example.com/todo/level09/internal/modules/activity/internal/application/mocks"
)

// newRouter mounts the route on a mocked driving port: these tests cover HTTP concerns only.
func newRouter(t *testing.T) (*gin.Engine, *mocks.MockActivityQueries) {
	gin.SetMode(gin.TestMode)
	queries := mocks.NewMockActivityQueries(t)
	router := gin.New()
	activityhttp.RegisterRoutes(router, queries, zerolog.Nop())
	return router, queries
}

func get(router *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestListActivities(t *testing.T) {
	router, queries := newRouter(t)
	id := uuid.NewString()
	activity := &application.Activity{ID: uuid.NewString(), TodoID: id, Action: "created", CreatedAt: time.Now().UTC()}
	queries.EXPECT().ListActivities(mock.Anything, id).Return([]*application.Activity{activity}, nil).Once()

	rec := get(router, "/todos/"+id+"/activities")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"action":"created"`)
	require.Contains(t, rec.Body.String(), `"todo_id":"`+id+`"`)
	require.Contains(t, rec.Body.String(), `"created_at":"`)
}

func TestListActivitiesEmpty(t *testing.T) {
	router, queries := newRouter(t)
	id := uuid.NewString()
	queries.EXPECT().ListActivities(mock.Anything, id).Return(nil, nil).Once()

	rec := get(router, "/todos/"+id+"/activities")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[]}`, rec.Body.String())
}

// The handler passes the raw :id on; the service's ErrNotFound for a malformed id is a 404.
func TestMalformedIDIsNotFound(t *testing.T) {
	router, queries := newRouter(t)
	queries.EXPECT().ListActivities(mock.Anything, "not-a-uuid").Return(nil, application.ErrNotFound).Once()

	rec := get(router, "/todos/not-a-uuid/activities")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"todo not found"}`, rec.Body.String())
}

func TestIDIsPassedAsGiven(t *testing.T) {
	router, queries := newRouter(t)
	id := strings.ToUpper(uuid.NewString())
	queries.EXPECT().ListActivities(mock.Anything, id).Return(nil, nil).Once()

	rec := get(router, "/todos/"+id+"/activities")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUnexpectedErrorIsHidden(t *testing.T) {
	router, queries := newRouter(t)
	id := uuid.NewString()
	queries.EXPECT().ListActivities(mock.Anything, id).Return(nil, errors.New("connection refused")).Once()

	rec := get(router, "/todos/"+id+"/activities")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}
