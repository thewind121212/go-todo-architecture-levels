package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	todohttp "example.com/todo/level06/internal/todo/adapters/http"
	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
	"example.com/todo/level06/internal/todo/ports/mocks"
)

// newRouter mounts the routes on a mocked driving port: these tests cover
// HTTP concerns only (parsing, status codes, JSON), not business rules.
func newRouter(t *testing.T) (*gin.Engine, *mocks.MockTodoService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockTodoService(t)
	router := gin.New()
	todohttp.RegisterRoutes(router, svc, zerolog.Nop())
	return router, svc
}

func do(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func sampleTodo(t *testing.T) *domain.Todo {
	t.Helper()
	todo, err := domain.NewTodo("buy milk")
	require.NoError(t, err)
	return todo
}

func TestCreate(t *testing.T) {
	router, svc := newRouter(t)
	todo := sampleTodo(t)
	svc.EXPECT().Create(mock.Anything, "buy milk").Return(todo, nil).Once()

	rec := do(router, http.MethodPost, "/todos", `{"title":"buy milk"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":"`+todo.ID+`"`)
	require.Contains(t, rec.Body.String(), `"completed":false`)
	require.Contains(t, rec.Body.String(), `"created_at":"`)
}

func TestCreateErrors(t *testing.T) {
	router, svc := newRouter(t)
	svc.EXPECT().Create(mock.Anything, " ").Return(nil, domain.ErrInvalidTitle).Once()

	rec := do(router, http.MethodPost, "/todos", `{"title":" "}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"title must be 1-200 characters"}`, rec.Body.String())

	rec = do(router, http.MethodPost, "/todos", `{"title":`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"invalid JSON body"}`, rec.Body.String())
}

func TestListDefaultsAndEmptyItems(t *testing.T) {
	router, svc := newRouter(t)
	svc.EXPECT().List(mock.Anything, 20, 0).Return(nil, nil).Once()

	rec := do(router, http.MethodGet, "/todos", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, rec.Body.String())
}

func TestListPassesPage(t *testing.T) {
	router, svc := newRouter(t)
	svc.EXPECT().List(mock.Anything, 5, 10).Return([]*domain.Todo{sampleTodo(t)}, nil).Once()

	rec := do(router, http.MethodGet, "/todos?limit=5&offset=10", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"limit":5,"offset":10`)
}

func TestListRejectsBadPage(t *testing.T) {
	router, _ := newRouter(t)
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "offset=-1", "offset=x"} {
		rec := do(router, http.MethodGet, "/todos?"+query, "")
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
		require.Contains(t, rec.Body.String(), `"error":`, query)
	}
}

func TestGetNotFound(t *testing.T) {
	router, svc := newRouter(t)
	svc.EXPECT().Get(mock.Anything, "not-a-uuid").Return(nil, domain.ErrNotFound).Once()

	rec := do(router, http.MethodGet, "/todos/not-a-uuid", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"todo not found"}`, rec.Body.String())
}

func TestUpdatePassesOnlyGivenFields(t *testing.T) {
	router, svc := newRouter(t)
	todo := sampleTodo(t)
	onlyCompleted := mock.MatchedBy(func(in ports.UpdateTodoInput) bool {
		return in.Title == nil && in.Completed != nil && *in.Completed
	})
	svc.EXPECT().Update(mock.Anything, todo.ID, onlyCompleted).Return(todo, nil).Once()

	rec := do(router, http.MethodPatch, "/todos/"+todo.ID, `{"completed":true}`)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdateEmptyBody(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Update(mock.Anything, id, ports.UpdateTodoInput{}).Return(nil, domain.ErrEmptyUpdate).Once()

	rec := do(router, http.MethodPatch, "/todos/"+id, `{}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"error":`)
}

func TestDelete(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Delete(mock.Anything, id).Return(nil).Once()

	rec := do(router, http.MethodDelete, "/todos/"+id, "")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestListActivitiesEmpty(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().ListActivities(mock.Anything, id).Return(nil, nil).Once()

	rec := do(router, http.MethodGet, "/todos/"+id+"/activities", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[]}`, rec.Body.String())
}

func TestUnexpectedErrorIsHidden(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Get(mock.Anything, id).Return(nil, errors.New("connection refused")).Once()

	rec := do(router, http.MethodGet, "/todos/"+id, "")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"error":"internal error"}`, rec.Body.String())
}
