package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func TestHandlerList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&fakeRepo{})
	router := gin.New()
	NewHandler(svc, zerolog.Nop()).Register(router)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	for _, path := range []string{"/todos/" + todoID + "/activities", "/todos/nope/activities"} {
		if w := get(path); w.Code != http.StatusOK || w.Body.String() != `{"items":[]}` {
			t.Errorf("GET %s = %d %s, want 200 {\"items\":[]}", path, w.Code, w.Body)
		}
	}

	if err := svc.Record(context.Background(), todoID, "created"); err != nil {
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
