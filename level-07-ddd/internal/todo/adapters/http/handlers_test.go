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

	todohttp "example.com/todo/level07/internal/todo/adapters/http"
	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
	"example.com/todo/level07/internal/todo/ports/mocks"
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
	title, err := domain.NewTitle("buy milk")
	require.NoError(t, err)
	return domain.NewTodo(domain.NewTodoID(), title, time.Now().UTC())
}

func TestCreate(t *testing.T) {
	router, svc := newRouter(t)
	todo := sampleTodo(t)
	svc.EXPECT().Create(mock.Anything, "buy milk").Return(todo, nil).Once()

	rec := do(router, http.MethodPost, "/todos", `{"title":"buy milk"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":"`+todo.ID().String()+`"`)
	require.Contains(t, rec.Body.String(), `"completed":false`)
	require.Contains(t, rec.Body.String(), `"created_at":"`)
}

func TestCreateErrors(t *testing.T) {
	router, svc := newRouter(t)
	svc.EXPECT().Create(mock.Anything, " ").Return(nil, domain.ErrInvalidTitle).Once()

	rec := do(router, http.MethodPost, "/todos", `{"title":" "}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"title must be 1 to 200 characters"}`, rec.Body.String())

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
	tests := map[string]string{
		"limit=0":   "limit must be between 1 and 100",
		"limit=101": "limit must be between 1 and 100",
		"limit=abc": "limit must be between 1 and 100",
		"offset=-1": "offset must be 0 or more",
		"offset=x":  "offset must be 0 or more",
	}
	for query, msg := range tests {
		rec := do(router, http.MethodGet, "/todos?"+query, "")
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
		require.JSONEq(t, `{"error":"`+msg+`"}`, rec.Body.String(), query)
	}
}

func TestGetNotFound(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Get(mock.Anything, id).Return(nil, domain.ErrNotFound).Once()

	rec := do(router, http.MethodGet, "/todos/"+id, "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"todo not found"}`, rec.Body.String())
}

// The application parses ids now, so the handler passes the raw :id on, and
// writeError turns the domain's ErrInvalidID into the same 404 as an unknown id.
func TestMalformedIDIsNotFound(t *testing.T) {
	router, svc := newRouter(t)
	const bad = "not-a-uuid"
	svc.EXPECT().Get(mock.Anything, bad).Return(nil, domain.ErrInvalidID).Once()
	svc.EXPECT().Update(mock.Anything, ports.UpdateTodoInput{ID: bad, Completed: new(true)}).Return(nil, domain.ErrInvalidID).Once()
	svc.EXPECT().Delete(mock.Anything, bad).Return(domain.ErrInvalidID).Once()
	svc.EXPECT().ListActivities(mock.Anything, bad).Return(nil, domain.ErrInvalidID).Once()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/todos/" + bad, ""},
		{http.MethodPatch, "/todos/" + bad, `{"completed":true}`},
		{http.MethodDelete, "/todos/" + bad, ""},
		{http.MethodGet, "/todos/" + bad + "/activities", ""},
	} {
		rec := do(router, tc.method, tc.path, tc.body)
		require.Equal(t, http.StatusNotFound, rec.Code, tc.method+" "+tc.path)
		require.JSONEq(t, `{"error":"todo not found"}`, rec.Body.String(), tc.method+" "+tc.path)
	}
}

func TestIDIsPassedAsGiven(t *testing.T) {
	router, svc := newRouter(t)
	id := strings.ToUpper(uuid.NewString())
	svc.EXPECT().Get(mock.Anything, id).Return(sampleTodo(t), nil).Once()

	rec := do(router, http.MethodGet, "/todos/"+id, "")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdatePassesOnlyGivenFields(t *testing.T) {
	router, svc := newRouter(t)
	todo := sampleTodo(t)
	onlyCompleted := mock.MatchedBy(func(in ports.UpdateTodoInput) bool {
		return in.ID == todo.ID().String() && in.Title == nil && in.Completed != nil && *in.Completed
	})
	svc.EXPECT().Update(mock.Anything, onlyCompleted).Return(todo, nil).Once()

	rec := do(router, http.MethodPatch, "/todos/"+todo.ID().String(), `{"completed":true}`)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdateEmptyBody(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Update(mock.Anything, ports.UpdateTodoInput{ID: id}).Return(nil, domain.ErrEmptyPatch).Once()

	rec := do(router, http.MethodPatch, "/todos/"+id, `{}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"patch must set title or completed"}`, rec.Body.String())
}

func TestUpdateConflict(t *testing.T) {
	router, svc := newRouter(t)
	id := uuid.NewString()
	svc.EXPECT().Update(mock.Anything, ports.UpdateTodoInput{ID: id, Completed: new(true)}).Return(nil, domain.ErrConflict).Once()

	rec := do(router, http.MethodPatch, "/todos/"+id, `{"completed":true}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.JSONEq(t, `{"error":"todo was changed by another request"}`, rec.Body.String())
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
