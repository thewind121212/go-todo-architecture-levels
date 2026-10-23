package main

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// fakeStore is a hand-written in-memory store. It follows the same flow as the
// real stores (newTodo, validatePatch, applyPatch) without a database.
type fakeStore struct {
	todos      map[string]Todo
	activities []Activity
	err        error // when set, every method fails with it
}

func newFakeStore() *fakeStore {
	return &fakeStore{todos: map[string]Todo{}}
}

func (f *fakeStore) Create(_ context.Context, title string) (Todo, error) {
	if f.err != nil {
		return Todo{}, f.err
	}
	todo, err := newTodo(title)
	if err != nil {
		return Todo{}, err
	}
	f.todos[todo.ID] = todo
	f.activities = append(f.activities, newActivity(todo.ID, ActionCreated, todo.CreatedAt))
	return todo, nil
}

func (f *fakeStore) List(_ context.Context, limit, offset int) ([]Todo, error) {
	if f.err != nil {
		return nil, f.err
	}
	// Newest first: UUIDv7 strings sort by creation time.
	todos := slices.SortedFunc(maps.Values(f.todos), func(a, b Todo) int { return strings.Compare(b.ID, a.ID) })
	if offset >= len(todos) {
		return []Todo{}, nil
	}
	return todos[offset:min(offset+limit, len(todos))], nil
}

func (f *fakeStore) Get(_ context.Context, id string) (Todo, error) {
	if f.err != nil {
		return Todo{}, f.err
	}
	todo, ok := f.todos[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return todo, nil
}

func (f *fakeStore) Update(_ context.Context, id string, p todoPatch) (Todo, error) {
	if f.err != nil {
		return Todo{}, f.err
	}
	p, err := validatePatch(p)
	if err != nil {
		return Todo{}, err
	}
	todo, ok := f.todos[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	f.activities = append(f.activities, applyPatch(&todo, p, now())...)
	f.todos[id] = todo
	return todo, nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.todos[id]; !ok {
		return ErrNotFound
	}
	delete(f.todos, id)
	f.activities = append(f.activities, newActivity(id, ActionDeleted, now()))
	return nil
}

func (f *fakeStore) Activities(_ context.Context, todoID string) ([]Activity, error) {
	if f.err != nil {
		return nil, f.err
	}
	activities := []Activity{}
	for _, a := range f.activities {
		if a.TodoID == todoID {
			activities = append(activities, a)
		}
	}
	return activities, nil
}

func (f *fakeStore) Ping(context.Context) error {
	return f.err
}

// serve sends one request through the real router and returns the recorder.
func serve(s store, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newRouter(s).ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, code, rec.Body.String())
	}
}

func wantError(t *testing.T, rec *httptest.ResponseRecorder, code int, msg string) {
	t.Helper()
	wantStatus(t, rec, code)
	var body map[string]string
	decode(t, rec, &body)
	if body["error"] != msg {
		t.Errorf("error = %q, want %q", body["error"], msg)
	}
}

func mustCreate(t *testing.T, s *fakeStore, title string) Todo {
	t.Helper()
	todo, err := s.Create(context.Background(), title)
	if err != nil {
		t.Fatal(err)
	}
	return todo
}

func TestHealth(t *testing.T) {
	s := newFakeStore()
	rec := serve(s, http.MethodGet, "/health", "")
	wantStatus(t, rec, http.StatusOK)
	var body map[string]string
	decode(t, rec, &body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}

	s.err = errors.New("db down")
	wantError(t, serve(s, http.MethodGet, "/health", ""), http.StatusInternalServerError, "internal error")
}

func TestCreateTodo(t *testing.T) {
	rec := serve(newFakeStore(), http.MethodPost, "/todos", `{"title":"  buy milk  "}`)
	wantStatus(t, rec, http.StatusCreated)
	var todo Todo
	decode(t, rec, &todo)
	if todo.Title != "buy milk" || todo.Completed || todo.ID == "" {
		t.Errorf("unexpected todo %+v", todo)
	}

	bad := []struct{ name, body, msg string }{
		{"blank title", `{"title":"   "}`, ErrInvalidTitle.Error()},
		{"missing title", `{}`, ErrInvalidTitle.Error()},
		{"too long", `{"title":"` + strings.Repeat("x", 201) + `"}`, ErrInvalidTitle.Error()},
		{"invalid JSON", `{"title":`, ErrInvalidJSON.Error()},
		{"wrong type", `{"title":42}`, ErrInvalidJSON.Error()},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			wantError(t, serve(newFakeStore(), http.MethodPost, "/todos", tt.body), http.StatusBadRequest, tt.msg)
		})
	}
}

func TestListTodos(t *testing.T) {
	t.Run("empty list is [] with defaults", func(t *testing.T) {
		rec := serve(newFakeStore(), http.MethodGet, "/todos", "")
		wantStatus(t, rec, http.StatusOK)
		if got := rec.Body.String(); got != `{"items":[],"limit":20,"offset":0}` {
			t.Errorf("body = %s", got)
		}
	})

	t.Run("newest first, paged", func(t *testing.T) {
		s := newFakeStore()
		first, second, third := mustCreate(t, s, "1"), mustCreate(t, s, "2"), mustCreate(t, s, "3")

		rec := serve(s, http.MethodGet, "/todos?limit=2&offset=0", "")
		wantStatus(t, rec, http.StatusOK)
		var page struct {
			Items  []Todo `json:"items"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		decode(t, rec, &page)
		if page.Limit != 2 || page.Offset != 0 || len(page.Items) != 2 ||
			page.Items[0].ID != third.ID || page.Items[1].ID != second.ID {
			t.Errorf("unexpected page %+v", page)
		}

		rec = serve(s, http.MethodGet, "/todos?limit=2&offset=2", "")
		wantStatus(t, rec, http.StatusOK)
		page.Items = nil
		decode(t, rec, &page)
		if len(page.Items) != 1 || page.Items[0].ID != first.ID {
			t.Errorf("unexpected page %+v", page)
		}
	})

	bad := []struct{ query, msg string }{
		{"limit=0", ErrInvalidLimit.Error()},
		{"limit=101", ErrInvalidLimit.Error()},
		{"limit=abc", ErrInvalidLimit.Error()},
		{"offset=-1", ErrInvalidOffset.Error()},
		{"offset=x", ErrInvalidOffset.Error()},
	}
	for _, tt := range bad {
		t.Run(tt.query, func(t *testing.T) {
			wantError(t, serve(newFakeStore(), http.MethodGet, "/todos?"+tt.query, ""), http.StatusBadRequest, tt.msg)
		})
	}
}

func TestGetTodo(t *testing.T) {
	s := newFakeStore()
	todo := mustCreate(t, s, "read")

	rec := serve(s, http.MethodGet, "/todos/"+todo.ID, "")
	wantStatus(t, rec, http.StatusOK)
	var got Todo
	decode(t, rec, &got)
	if got != todo {
		t.Errorf("got %+v, want %+v", got, todo)
	}

	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		wantError(t, serve(s, http.MethodGet, "/todos/"+id, ""), http.StatusNotFound, "todo not found")
	}
}

func TestUpdateTodo(t *testing.T) {
	s := newFakeStore()
	todo := mustCreate(t, s, "old")

	rec := serve(s, http.MethodPatch, "/todos/"+todo.ID, `{"title":" new ","completed":true}`)
	wantStatus(t, rec, http.StatusOK)
	var got Todo
	decode(t, rec, &got)
	if got.Title != "new" || !got.Completed {
		t.Errorf("unexpected todo %+v", got)
	}

	bad := []struct {
		name, id, body string
		code           int
		msg            string
	}{
		{"empty patch", todo.ID, `{}`, http.StatusBadRequest, ErrEmptyPatch.Error()},
		{"blank title", todo.ID, `{"title":" "}`, http.StatusBadRequest, ErrInvalidTitle.Error()},
		{"invalid JSON", todo.ID, `nope`, http.StatusBadRequest, ErrInvalidJSON.Error()},
		{"unknown id", uuid.NewString(), `{"completed":true}`, http.StatusNotFound, "todo not found"},
		{"malformed id", "123", `{"completed":true}`, http.StatusNotFound, "todo not found"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			wantError(t, serve(s, http.MethodPatch, "/todos/"+tt.id, tt.body), tt.code, tt.msg)
		})
	}
}

func TestDeleteTodo(t *testing.T) {
	s := newFakeStore()
	todo := mustCreate(t, s, "bye")

	rec := serve(s, http.MethodDelete, "/todos/"+todo.ID, "")
	wantStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Errorf("204 must have no body, got %q", rec.Body.String())
	}
	wantError(t, serve(s, http.MethodDelete, "/todos/"+todo.ID, ""), http.StatusNotFound, "todo not found")
}

func TestListActivities(t *testing.T) {
	s := newFakeStore()
	todo := mustCreate(t, s, "track me")
	serve(s, http.MethodPatch, "/todos/"+todo.ID, `{"completed":true}`)

	rec := serve(s, http.MethodGet, "/todos/"+todo.ID+"/activities", "")
	wantStatus(t, rec, http.StatusOK)
	var page struct{ Items []Activity }
	decode(t, rec, &page)
	var actions []string
	for _, a := range page.Items {
		actions = append(actions, a.Action)
	}
	if !slices.Equal(actions, []string{ActionCreated, ActionCompleted}) {
		t.Errorf("actions = %v", actions)
	}

	rec = serve(s, http.MethodGet, "/todos/"+uuid.NewString()+"/activities", "")
	wantStatus(t, rec, http.StatusOK)
	if got := rec.Body.String(); got != `{"items":[]}` {
		t.Errorf("body = %s, want empty items", got)
	}
}

func TestStoreErrorIsInternal(t *testing.T) {
	s := newFakeStore()
	s.err = errors.New("connection reset")

	rec := serve(s, http.MethodPost, "/todos", `{"title":"x"}`)
	wantError(t, rec, http.StatusInternalServerError, "internal error")
	if strings.Contains(rec.Body.String(), "connection reset") {
		t.Error("internal error details must not leak to the client")
	}
}
