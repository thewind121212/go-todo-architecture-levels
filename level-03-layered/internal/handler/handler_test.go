package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"example.com/todo/level03/internal/handler"
	"example.com/todo/level03/internal/repository/fake"
	"example.com/todo/level03/internal/service"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

type todoJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// testServer runs the real service on fake repositories.
type testServer struct {
	router     *gin.Engine
	activities *fake.ActivityRepository
	pingErr    error
}

func newTestServer() *testServer {
	s := &testServer{activities: &fake.ActivityRepository{}}
	svc := service.NewTodoService(&fake.TxManager{}, fake.NewTodoRepository(), s.activities)
	ping := func(context.Context) error { return s.pingErr }
	s.router = handler.NewRouter(svc, ping, zerolog.Nop())
	return s
}

func (s *testServer) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func (s *testServer) createTodo(t *testing.T, title string) todoJSON {
	t.Helper()
	rec := s.do(t, http.MethodPost, "/todos", `{"title":"`+title+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", rec.Code, rec.Body)
	}
	var todo todoJSON
	decode(t, rec, &todo)
	return todo
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body)
	}
	var body struct {
		Error string `json:"error"`
	}
	decode(t, rec, &body)
	if body.Error != wantMsg {
		t.Errorf("error = %q, want %q", body.Error, wantMsg)
	}
}

func TestHealth(t *testing.T) {
	s := newTestServer()
	if rec := s.do(t, http.MethodGet, "/health", ""); rec.Code != http.StatusOK || rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("got %d %s, want 200 {\"status\":\"ok\"}", rec.Code, rec.Body)
	}

	s.pingErr = errors.New("db down")
	assertError(t, s.do(t, http.MethodGet, "/health", ""), http.StatusInternalServerError, "internal error")
}

func TestCreateTodo(t *testing.T) {
	s := newTestServer()

	got := s.createTodo(t, "  buy milk ")
	if got.Title != "buy milk" || got.Completed || got.ID == "" || got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Errorf("got %+v, want trimmed, not completed, with id and timestamps", got)
	}
}

func TestBadRequests(t *testing.T) {
	s := newTestServer()
	todo := s.createTodo(t, "title")

	tests := []struct {
		name, method, path, body, wantMsg string
	}{
		{"blank title", http.MethodPost, "/todos", `{"title":"  "}`, "title must be 1-200 characters"},
		{"title too long", http.MethodPost, "/todos", `{"title":"` + strings.Repeat("x", 201) + `"}`, "title must be 1-200 characters"},
		{"invalid json", http.MethodPost, "/todos", `{`, "invalid JSON body"},
		{"wrong type", http.MethodPatch, "/todos/" + todo.ID, `{"completed":"yes"}`, "invalid JSON body"},
		{"empty patch", http.MethodPatch, "/todos/" + todo.ID, `{}`, "provide title and/or completed"},
		{"limit 0", http.MethodGet, "/todos?limit=0", "", "limit must be an integer from 1 to 100"},
		{"limit 101", http.MethodGet, "/todos?limit=101", "", "limit must be an integer from 1 to 100"},
		{"limit abc", http.MethodGet, "/todos?limit=abc", "", "limit must be an integer from 1 to 100"},
		{"offset -1", http.MethodGet, "/todos?offset=-1", "", "offset must be an integer >= 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, s.do(t, tt.method, tt.path, tt.body), http.StatusBadRequest, tt.wantMsg)
		})
	}
}

func TestNotFound(t *testing.T) {
	s := newTestServer()
	unknown := uuid.NewString()

	tests := []struct {
		name, method, path, body string
	}{
		{"get unknown", http.MethodGet, "/todos/" + unknown, ""},
		{"get malformed", http.MethodGet, "/todos/not-a-uuid", ""},
		{"patch unknown", http.MethodPatch, "/todos/" + unknown, `{"completed":true}`},
		{"delete unknown", http.MethodDelete, "/todos/" + unknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, s.do(t, tt.method, tt.path, tt.body), http.StatusNotFound, "todo not found")
		})
	}
}

func TestListTodos(t *testing.T) {
	s := newTestServer()

	rec := s.do(t, http.MethodGet, "/todos", "")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"items":[],"limit":20,"offset":0}` {
		t.Fatalf("empty list: got %d %s", rec.Code, rec.Body)
	}

	first := s.createTodo(t, "first")
	second := s.createTodo(t, "second")
	var page struct {
		Items  []todoJSON `json:"items"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}
	decode(t, s.do(t, http.MethodGet, "/todos?limit=1&offset=1", ""), &page)
	if page.Limit != 1 || page.Offset != 1 || len(page.Items) != 1 || page.Items[0].ID != first.ID {
		t.Errorf("page = %+v, want only %q (newest first, %q skipped)", page, first.ID, second.ID)
	}
}

func TestUpdateDeleteAndActivities(t *testing.T) {
	s := newTestServer()
	todo := s.createTodo(t, "old")

	rec := s.do(t, http.MethodPatch, "/todos/"+todo.ID, `{"title":"new","completed":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", rec.Code, rec.Body)
	}
	var got todoJSON
	decode(t, rec, &got)
	if got.Title != "new" || !got.Completed {
		t.Errorf("patch: got %+v, want title new and completed", got)
	}

	if rec := s.do(t, http.MethodDelete, "/todos/"+todo.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status = %d, want 204", rec.Code)
	}

	var activities struct {
		Items []struct {
			Action string `json:"action"`
		} `json:"items"`
	}
	decode(t, s.do(t, http.MethodGet, "/todos/"+todo.ID+"/activities", ""), &activities)
	var actions []string
	for _, a := range activities.Items {
		actions = append(actions, a.Action)
	}
	if strings.Join(actions, ",") != "created,renamed,completed,deleted" {
		t.Errorf("actions = %v, want created,renamed,completed,deleted", actions)
	}
}

func TestListActivitiesUnknownTodoIsEmpty(t *testing.T) {
	s := newTestServer()
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		rec := s.do(t, http.MethodGet, "/todos/"+id+"/activities", "")
		if rec.Code != http.StatusOK || rec.Body.String() != `{"items":[]}` {
			t.Errorf("%s: got %d %s, want 200 {\"items\":[]}", id, rec.Code, rec.Body)
		}
	}
}

func TestInternalErrorIsHidden(t *testing.T) {
	s := newTestServer()
	s.activities.Err = errors.New("connection reset")

	assertError(t, s.do(t, http.MethodPost, "/todos", `{"title":"x"}`), http.StatusInternalServerError, "internal error")
}
