package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestHandlerList(t *testing.T) {
	svc := NewService(&fakeRepo{})
	router := gin.New()
	NewHandler(svc, zerolog.Nop()).Register(router)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	if w := get("/todos/" + todoID + "/activities"); w.Code != http.StatusOK || w.Body.String() != `{"items":[]}` {
		t.Errorf("unknown todo: got %d %s, want 200 {\"items\":[]}", w.Code, w.Body)
	}
	if w := get("/todos/not-a-uuid/activities"); w.Code != http.StatusNotFound || w.Body.String() != `{"error":"todo not found"}` {
		t.Errorf("malformed id: got %d %s, want 404 todo not found", w.Code, w.Body)
	}

	if err := svc.Record(context.Background(), todoID, "created", time.Now().UTC()); err != nil {
		t.Fatalf("Record: %v", err)
	}
	w := get("/todos/" + todoID + "/activities")
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	if len(body.Items) != 1 || body.Items[0]["action"] != "created" || body.Items[0]["todo_id"] != todoID {
		t.Errorf("body = %s", w.Body)
	}
}
