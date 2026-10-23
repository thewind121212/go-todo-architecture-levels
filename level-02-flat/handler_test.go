// handler_test.go: handler tests through the real router, backed by an in-memory fake store.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// fakeStore is a hand-written in-memory store. It follows the same rules as
// the real stores (it even calls applyPatch), but needs no database.
type fakeStore struct {
	mu         sync.Mutex
	todos      map[string]Todo
	activities []Activity
	err        error // when set, every method fails with it
}

func newFakeStore() *fakeStore {
	return &fakeStore{todos: map[string]Todo{}}
}

func (f *fakeStore) Create(_ context.Context, t Todo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.todos[t.ID] = t
	f.activities = append(f.activities, newActivity(t.ID, ActionCreated, t.CreatedAt))
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (Todo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Todo{}, f.err
	}
	t, ok := f.todos[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) List(_ context.Context, limit, offset int) ([]Todo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	todos := []Todo{}
	for _, t := range f.todos {
		todos = append(todos, t)
	}
	slices.SortFunc(todos, func(a, b Todo) int { return strings.Compare(b.ID, a.ID) })
	todos = todos[min(offset, len(todos)):]
	return todos[:min(limit, len(todos))], nil
}

func (f *fakeStore) Update(_ context.Context, id string, p Patch) (Todo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return Todo{}, f.err
	}
	t, ok := f.todos[id]
	if !ok {
		return Todo{}, ErrNotFound
	}
	f.activities = append(f.activities, applyPatch(&t, p, nowUTC())...)
	f.todos[id] = t
	return t, nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if _, ok := f.todos[id]; !ok {
		return ErrNotFound
	}
	delete(f.todos, id)
	f.activities = append(f.activities, newActivity(id, ActionDeleted, nowUTC()))
	return nil
}

func (f *fakeStore) ListActivities(_ context.Context, todoID string) ([]Activity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// do sends one request through the real router and decodes the JSON reply into out (if not nil).
func do(t *testing.T, router http.Handler, method, path, body string, out any) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code
}

type errorBody struct {
	Error string `json:"error"`
}

func TestHandlersHappyPath(t *testing.T) {
	router := newRouter(newFakeStore())

	var created Todo
	if code := do(t, router, "POST", "/todos", `{"title":"  buy milk  "}`, &created); code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	if created.Title != "buy milk" || created.Completed || created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("create: got %+v", created)
	}

	var got Todo
	if code := do(t, router, "GET", "/todos/"+created.ID, "", &got); code != http.StatusOK || got != created {
		t.Fatalf("get: status %d, got %+v, want %+v", code, got, created)
	}

	var page struct {
		Items  []Todo `json:"items"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if code := do(t, router, "GET", "/todos", "", &page); code != http.StatusOK {
		t.Fatalf("list: status %d", code)
	}
	if len(page.Items) != 1 || page.Limit != 20 || page.Offset != 0 {
		t.Fatalf("list: got %+v", page)
	}

	var updated Todo
	code := do(t, router, "PATCH", "/todos/"+created.ID, `{"title":"buy oat milk","completed":true}`, &updated)
	if code != http.StatusOK || updated.Title != "buy oat milk" || !updated.Completed {
		t.Fatalf("patch: status %d, got %+v", code, updated)
	}

	if code := do(t, router, "DELETE", "/todos/"+created.ID, "", nil); code != http.StatusNoContent {
		t.Fatalf("delete: status %d", code)
	}

	var acts struct {
		Items []Activity `json:"items"`
	}
	if code := do(t, router, "GET", "/todos/"+created.ID+"/activities", "", &acts); code != http.StatusOK {
		t.Fatalf("activities: status %d", code)
	}
	var actions []Action
	for _, a := range acts.Items {
		actions = append(actions, a.Action)
	}
	want := []Action{ActionCreated, ActionRenamed, ActionCompleted, ActionDeleted}
	if !slices.Equal(actions, want) {
		t.Fatalf("activities = %v, want %v", actions, want)
	}
}

func TestHandlersEmptyListsAreArrays(t *testing.T) {
	router := newRouter(newFakeStore())
	for _, path := range []string{"/todos", "/todos/" + newID() + "/activities"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("GET %s = %s, want items to be []", path, rec.Body.String())
		}
	}
}

func TestHandlersErrors(t *testing.T) {
	fake := newFakeStore()
	router := newRouter(fake)
	existing, _ := newTodo("existing")
	if err := fake.Create(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	missing := newID()

	tests := []struct {
		name, method, path, body string
		wantCode                 int
		wantError                string
	}{
		{"invalid json", "POST", "/todos", `{"title":`, 400, ErrInvalidJSON.Error()},
		{"blank title", "POST", "/todos", `{"title":"  "}`, 400, ErrInvalidTitle.Error()},
		{"missing title", "POST", "/todos", `{}`, 400, ErrInvalidTitle.Error()},
		{"too long title", "POST", "/todos", `{"title":"` + strings.Repeat("x", 201) + `"}`, 400, ErrInvalidTitle.Error()},
		{"limit 0", "GET", "/todos?limit=0", "", 400, ErrInvalidLimit.Error()},
		{"limit 101", "GET", "/todos?limit=101", "", 400, ErrInvalidLimit.Error()},
		{"limit abc", "GET", "/todos?limit=abc", "", 400, ErrInvalidLimit.Error()},
		{"offset -1", "GET", "/todos?offset=-1", "", 400, ErrInvalidOffset.Error()},
		{"get unknown", "GET", "/todos/" + missing, "", 404, "todo not found"},
		{"get malformed", "GET", "/todos/not-a-uuid", "", 404, "todo not found"},
		{"patch empty", "PATCH", "/todos/" + existing.ID, `{}`, 400, ErrEmptyPatch.Error()},
		{"patch invalid json", "PATCH", "/todos/" + existing.ID, `nope`, 400, ErrInvalidJSON.Error()},
		{"patch blank title", "PATCH", "/todos/" + existing.ID, `{"title":""}`, 400, ErrInvalidTitle.Error()},
		{"patch unknown", "PATCH", "/todos/" + missing, `{"completed":true}`, 404, "todo not found"},
		{"delete unknown", "DELETE", "/todos/" + missing, "", 404, "todo not found"},
		{"activities malformed", "GET", "/todos/xyz/activities", "", 404, "todo not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body errorBody
			code := do(t, router, tt.method, tt.path, tt.body, &body)
			if code != tt.wantCode || body.Error != tt.wantError {
				t.Errorf("got %d %q, want %d %q", code, body.Error, tt.wantCode, tt.wantError)
			}
		})
	}
}

func TestHandlersHideInternalErrors(t *testing.T) {
	fake := newFakeStore()
	fake.err = errors.New("connection refused")
	router := newRouter(fake)

	for _, path := range []string{"/health", "/todos", "/todos/" + newID()} {
		var body errorBody
		code := do(t, router, "GET", path, "", &body)
		if code != http.StatusInternalServerError || body.Error != "internal error" {
			t.Errorf("GET %s: got %d %q, want 500 \"internal error\"", path, code, body.Error)
		}
	}
}
