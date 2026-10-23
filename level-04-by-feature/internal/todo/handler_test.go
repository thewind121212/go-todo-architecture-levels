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

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// newTestRouter serves the real handler and service on top of the fakes.
func newTestRouter(rec *fakeRecorder) *gin.Engine {
	router := gin.New()
	NewHandler(NewService(&fakeTx{}, newFakeRepo(), rec), zerolog.Nop()).Register(router)
	return router
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

type todoJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func createTodo(t *testing.T, router *gin.Engine, title string) todoJSON {
	t.Helper()
	w := serve(router, http.MethodPost, "/todos", `{"title":"`+title+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body %s", w.Code, w.Body)
	}
	var got todoJSON
	decode(t, w, &got)
	return got
}

func TestHandlerCreate(t *testing.T) {
	router := newTestRouter(&fakeRecorder{})

	got := createTodo(t, router, " buy milk ")
	if got.Title != "buy milk" || got.Completed || got.ID == "" || got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Errorf("got %+v, want trimmed, not completed, with id and timestamps", got)
	}
}

func TestHandlerErrors(t *testing.T) {
	router := newTestRouter(&fakeRecorder{})
	existing := createTodo(t, router, "x")

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantError  string
	}{
		{"blank title", http.MethodPost, "/todos", `{"title":"  "}`, 400, "title must be 1 to 200 characters"},
		{"invalid JSON", http.MethodPost, "/todos", `{`, 400, "invalid JSON body"},
		{"wrong type", http.MethodPatch, "/todos/" + existing.ID, `{"completed":"yes"}`, 400, "invalid JSON body"},
		{"empty patch", http.MethodPatch, "/todos/" + existing.ID, `{}`, 400, "patch must set title or completed"},
		{"empty patch on unknown id", http.MethodPatch, "/todos/" + unknownID, `{}`, 400, "patch must set title or completed"},
		{"limit 0", http.MethodGet, "/todos?limit=0", "", 400, "limit must be between 1 and 100"},
		{"limit 101", http.MethodGet, "/todos?limit=101", "", 400, "limit must be between 1 and 100"},
		{"limit abc", http.MethodGet, "/todos?limit=abc", "", 400, "limit must be between 1 and 100"},
		{"offset -1", http.MethodGet, "/todos?offset=-1", "", 400, "offset must be 0 or more"},
		{"get malformed id", http.MethodGet, "/todos/not-a-uuid", "", 404, "todo not found"},
		{"get unknown id", http.MethodGet, "/todos/" + unknownID, "", 404, "todo not found"},
		{"patch malformed id", http.MethodPatch, "/todos/not-a-uuid", `{"completed":true}`, 404, "todo not found"},
		{"patch unknown id", http.MethodPatch, "/todos/" + unknownID, `{"completed":true}`, 404, "todo not found"},
		{"delete malformed id", http.MethodDelete, "/todos/not-a-uuid", "", 404, "todo not found"},
		{"delete unknown id", http.MethodDelete, "/todos/" + unknownID, "", 404, "todo not found"},
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
	router := newTestRouter(&fakeRecorder{})

	w := serve(router, http.MethodGet, "/todos", "")
	if want := `{"items":[],"limit":20,"offset":0}`; w.Code != http.StatusOK || w.Body.String() != want {
		t.Fatalf("empty list: got %d %s, want 200 %s", w.Code, w.Body, want)
	}

	first := createTodo(t, router, "first")
	createTodo(t, router, "second")
	var page struct {
		Items  []todoJSON `json:"items"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}
	decode(t, serve(router, http.MethodGet, "/todos?limit=1&offset=1", ""), &page)
	if page.Limit != 1 || page.Offset != 1 || len(page.Items) != 1 || page.Items[0].ID != first.ID {
		t.Errorf("page = %+v, want only the older todo %q", page, first.ID)
	}
}

func TestHandlerUpdateAndDelete(t *testing.T) {
	router := newTestRouter(&fakeRecorder{})
	todo := createTodo(t, router, "x")

	w := serve(router, http.MethodPatch, "/todos/"+strings.ToUpper(todo.ID), `{"title":"y","completed":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body %s", w.Code, w.Body)
	}
	var got todoJSON
	decode(t, w, &got)
	if got.ID != todo.ID || got.Title != "y" || !got.Completed {
		t.Errorf("patched = %+v", got)
	}

	if w := serve(router, http.MethodDelete, "/todos/"+todo.ID, ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", w.Code)
	}
}

func TestHandlerHidesInternalErrors(t *testing.T) {
	router := newTestRouter(&fakeRecorder{err: errBoom})

	w := serve(router, http.MethodPost, "/todos", `{"title":"x"}`)
	if want := `{"error":"internal error"}`; w.Code != http.StatusInternalServerError || w.Body.String() != want {
		t.Errorf("got %d %s, want 500 %s", w.Code, w.Body, want)
	}
}
