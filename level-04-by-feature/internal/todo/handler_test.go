package todo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// newTestRouter serves the real handler and service on top of the fakes.
func newTestRouter(rec *fakeRecorder) (*gin.Engine, *fakeRepo) {
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo()
	router := gin.New()
	NewHandler(NewService(repo, fakeTx{}, rec), zerolog.Nop()).Register(router)
	return router, repo
}

func serve(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
}

func TestHandlerCreate(t *testing.T) {
	router, _ := newTestRouter(&fakeRecorder{})

	w := serve(router, http.MethodPost, "/todos", `{"title":" buy milk "}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got map[string]any
	decode(t, w, &got)
	for _, key := range []string{"id", "title", "completed", "created_at", "updated_at"} {
		if _, ok := got[key]; !ok {
			t.Errorf("response has no %q: %s", key, w.Body)
		}
	}
	if got["title"] != "buy milk" {
		t.Errorf("title = %v", got["title"])
	}
}

func TestHandlerErrors(t *testing.T) {
	router, repo := newTestRouter(&fakeRecorder{})
	existing, _ := New("x")
	repo.todos[existing.ID] = existing

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantError  string
	}{
		{"blank title", http.MethodPost, "/todos", `{"title":"  "}`, 400, ErrInvalidTitle.Error()},
		{"invalid JSON", http.MethodPost, "/todos", `{`, 400, errInvalidJSON.Error()},
		{"limit zero", http.MethodGet, "/todos?limit=0", "", 400, ErrInvalidLimit.Error()},
		{"limit too big", http.MethodGet, "/todos?limit=101", "", 400, ErrInvalidLimit.Error()},
		{"limit not a number", http.MethodGet, "/todos?limit=abc", "", 400, ErrInvalidLimit.Error()},
		{"negative offset", http.MethodGet, "/todos?offset=-1", "", 400, ErrInvalidOffset.Error()},
		{"malformed id", http.MethodGet, "/todos/not-a-uuid", "", 404, ErrNotFound.Error()},
		{"unknown id", http.MethodGet, "/todos/0190a0e4-0000-7000-8000-000000000000", "", 404, ErrNotFound.Error()},
		{"empty patch", http.MethodPatch, "/todos/" + existing.ID, `{}`, 400, ErrEmptyUpdate.Error()},
		{"delete unknown", http.MethodDelete, "/todos/0190a0e4-0000-7000-8000-000000000000", "", 404, ErrNotFound.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(router, tt.method, tt.path, tt.body)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body)
			}
			var body map[string]string
			decode(t, w, &body)
			if body["error"] != tt.wantError {
				t.Errorf("error = %q, want %q", body["error"], tt.wantError)
			}
		})
	}
}

func TestHandlerList(t *testing.T) {
	router, _ := newTestRouter(&fakeRecorder{})

	w := serve(router, http.MethodGet, "/todos", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if want := `{"items":[],"limit":20,"offset":0}`; w.Body.String() != want {
		t.Errorf("body = %s, want %s", w.Body, want)
	}
}

func TestHandlerUpdateAndDelete(t *testing.T) {
	router, _ := newTestRouter(&fakeRecorder{})
	var created Todo
	decode(t, serve(router, http.MethodPost, "/todos", `{"title":"x"}`), &created)

	w := serve(router, http.MethodPatch, "/todos/"+created.ID, `{"title":"y","completed":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", w.Code, w.Body)
	}
	var got Todo
	decode(t, w, &got)
	if got.Title != "y" || !got.Completed {
		t.Errorf("patched = %+v", got)
	}

	if w := serve(router, http.MethodDelete, "/todos/"+created.ID, ""); w.Code != http.StatusNoContent {
		t.Errorf("delete status = %d", w.Code)
	}
}

func TestHandlerHidesInternalErrors(t *testing.T) {
	router, _ := newTestRouter(&fakeRecorder{err: errBoom})

	w := serve(router, http.MethodPost, "/todos", `{"title":"x"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	if want := `{"error":"internal error"}`; w.Body.String() != want {
		t.Errorf("body = %s, want %s", w.Body, want)
	}
}
