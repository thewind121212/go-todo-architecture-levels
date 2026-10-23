package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

// Each fake is a func that satisfies a one-method input port.
type (
	createFunc         func(context.Context, usecase.CreateTodoInput) (usecase.TodoOutput, error)
	getFunc            func(context.Context, usecase.GetTodoInput) (usecase.TodoOutput, error)
	listFunc           func(context.Context, usecase.ListTodosInput) (usecase.ListTodosOutput, error)
	updateFunc         func(context.Context, usecase.UpdateTodoInput) (usecase.TodoOutput, error)
	deleteFunc         func(context.Context, usecase.DeleteTodoInput) error
	listActivitiesFunc func(context.Context, usecase.ListActivitiesInput) (usecase.ListActivitiesOutput, error)
)

func (f createFunc) Execute(ctx context.Context, in usecase.CreateTodoInput) (usecase.TodoOutput, error) {
	return f(ctx, in)
}

func (f getFunc) Execute(ctx context.Context, in usecase.GetTodoInput) (usecase.TodoOutput, error) {
	return f(ctx, in)
}

func (f listFunc) Execute(ctx context.Context, in usecase.ListTodosInput) (usecase.ListTodosOutput, error) {
	return f(ctx, in)
}

func (f updateFunc) Execute(ctx context.Context, in usecase.UpdateTodoInput) (usecase.TodoOutput, error) {
	return f(ctx, in)
}

func (f deleteFunc) Execute(ctx context.Context, in usecase.DeleteTodoInput) error {
	return f(ctx, in)
}

func (f listActivitiesFunc) Execute(ctx context.Context, in usecase.ListActivitiesInput) (usecase.ListActivitiesOutput, error) {
	return f(ctx, in)
}

// fakes holds one fake per use case; a test sets only the ones its route calls.
type fakes struct {
	create         createFunc
	get            getFunc
	list           listFunc
	update         updateFunc
	delete         deleteFunc
	listActivities listActivitiesFunc
	ping           func(context.Context) error
}

func (f fakes) serve(method, target, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	log := zerolog.Nop()
	todos := NewTodoController(f.create, f.get, f.list, f.update, f.delete, log)
	router := NewRouter(todos, NewActivityController(f.listActivities, log), f.ping, log)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Errorf("body = %s\nwant   %s", got, wantBody)
	}
}

var sampleTodo = usecase.TodoOutput{
	ID:        "0190a000-0000-7000-8000-000000000001",
	Title:     "buy milk",
	CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC),
	UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 6_000_000, time.UTC),
}

const sampleTodoJSON = `{"id":"0190a000-0000-7000-8000-000000000001","title":"buy milk","completed":false,` +
	`"created_at":"2026-01-02T03:04:05.006Z","updated_at":"2026-01-02T03:04:05.006Z"}`

func TestHealth(t *testing.T) {
	ok := fakes{ping: func(context.Context) error { return nil }}
	assertResponse(t, ok.serve("GET", "/health", ""), http.StatusOK, `{"status":"ok"}`)

	down := fakes{ping: func(context.Context) error { return errors.New("db down") }}
	assertResponse(t, down.serve("GET", "/health", ""), http.StatusInternalServerError, `{"error":"internal error"}`)
}

func TestCreateTodo(t *testing.T) {
	t.Run("201 with the presented todo", func(t *testing.T) {
		f := fakes{create: func(_ context.Context, in usecase.CreateTodoInput) (usecase.TodoOutput, error) {
			if in.Title != "buy milk" {
				t.Errorf("title = %q", in.Title)
			}
			return sampleTodo, nil
		}}
		assertResponse(t, f.serve("POST", "/todos", `{"title":"buy milk"}`), http.StatusCreated, sampleTodoJSON)
	})

	t.Run("invalid JSON is 400", func(t *testing.T) {
		assertResponse(t, fakes{}.serve("POST", "/todos", `{`), http.StatusBadRequest, `{"error":"invalid JSON body"}`)
	})

	t.Run("invalid title is 400", func(t *testing.T) {
		f := fakes{create: func(context.Context, usecase.CreateTodoInput) (usecase.TodoOutput, error) {
			return usecase.TodoOutput{}, entity.ErrInvalidTitle
		}}
		assertResponse(t, f.serve("POST", "/todos", `{"title":""}`), http.StatusBadRequest, `{"error":"title must be 1-200 characters"}`)
	})

	t.Run("unexpected error is a generic 500", func(t *testing.T) {
		f := fakes{create: func(context.Context, usecase.CreateTodoInput) (usecase.TodoOutput, error) {
			return usecase.TodoOutput{}, errors.New("connection reset")
		}}
		assertResponse(t, f.serve("POST", "/todos", `{"title":"x"}`), http.StatusInternalServerError, `{"error":"internal error"}`)
	})
}

func TestGetTodo(t *testing.T) {
	f := fakes{get: func(_ context.Context, in usecase.GetTodoInput) (usecase.TodoOutput, error) {
		if in.ID != sampleTodo.ID {
			return usecase.TodoOutput{}, entity.ErrNotFound
		}
		return sampleTodo, nil
	}}
	assertResponse(t, f.serve("GET", "/todos/"+sampleTodo.ID, ""), http.StatusOK, sampleTodoJSON)
	assertResponse(t, f.serve("GET", "/todos/other", ""), http.StatusNotFound, `{"error":"todo not found"}`)
}

func TestListTodos(t *testing.T) {
	t.Run("defaults and empty items", func(t *testing.T) {
		f := fakes{list: func(_ context.Context, in usecase.ListTodosInput) (usecase.ListTodosOutput, error) {
			if in.Limit != 20 || in.Offset != 0 {
				t.Errorf("input = %+v, want limit 20 offset 0", in)
			}
			return usecase.ListTodosOutput{Limit: in.Limit, Offset: in.Offset}, nil
		}}
		assertResponse(t, f.serve("GET", "/todos", ""), http.StatusOK, `{"items":[],"limit":20,"offset":0}`)
	})

	t.Run("passes the query to the use case", func(t *testing.T) {
		f := fakes{list: func(_ context.Context, in usecase.ListTodosInput) (usecase.ListTodosOutput, error) {
			return usecase.ListTodosOutput{Items: []usecase.TodoOutput{sampleTodo}, Limit: in.Limit, Offset: in.Offset}, nil
		}}
		assertResponse(t, f.serve("GET", "/todos?limit=5&offset=10", ""), http.StatusOK,
			`{"items":[`+sampleTodoJSON+`],"limit":5,"offset":10}`)
	})

	t.Run("non-integer query is 400", func(t *testing.T) {
		assertResponse(t, fakes{}.serve("GET", "/todos?limit=abc", ""), http.StatusBadRequest,
			`{"error":"limit and offset must be integers"}`)
	})

	t.Run("out of range is 400", func(t *testing.T) {
		f := fakes{list: func(context.Context, usecase.ListTodosInput) (usecase.ListTodosOutput, error) {
			return usecase.ListTodosOutput{}, usecase.ErrInvalidPagination
		}}
		assertResponse(t, f.serve("GET", "/todos?limit=0", ""), http.StatusBadRequest,
			`{"error":"limit must be 1-100 and offset must not be negative"}`)
	})
}

func TestUpdateTodo(t *testing.T) {
	t.Run("sent fields become non-nil inputs", func(t *testing.T) {
		f := fakes{update: func(_ context.Context, in usecase.UpdateTodoInput) (usecase.TodoOutput, error) {
			if in.ID != sampleTodo.ID || in.Title != nil || in.Completed == nil || !*in.Completed {
				t.Errorf("unexpected input: %+v", in)
			}
			return sampleTodo, nil
		}}
		assertResponse(t, f.serve("PATCH", "/todos/"+sampleTodo.ID, `{"completed":true}`), http.StatusOK, sampleTodoJSON)
	})

	t.Run("empty update is 400", func(t *testing.T) {
		f := fakes{update: func(context.Context, usecase.UpdateTodoInput) (usecase.TodoOutput, error) {
			return usecase.TodoOutput{}, usecase.ErrEmptyUpdate
		}}
		assertResponse(t, f.serve("PATCH", "/todos/x", `{}`), http.StatusBadRequest, `{"error":"provide title and/or completed"}`)
	})

	t.Run("wrong JSON type is 400", func(t *testing.T) {
		assertResponse(t, fakes{}.serve("PATCH", "/todos/x", `{"completed":"yes"}`), http.StatusBadRequest, `{"error":"invalid JSON body"}`)
	})
}

func TestDeleteTodo(t *testing.T) {
	f := fakes{delete: func(_ context.Context, in usecase.DeleteTodoInput) error {
		if in.ID != sampleTodo.ID {
			return entity.ErrNotFound
		}
		return nil
	}}
	assertResponse(t, f.serve("DELETE", "/todos/"+sampleTodo.ID, ""), http.StatusNoContent, "")
	assertResponse(t, f.serve("DELETE", "/todos/other", ""), http.StatusNotFound, `{"error":"todo not found"}`)
}

func TestListActivities(t *testing.T) {
	f := fakes{listActivities: func(_ context.Context, in usecase.ListActivitiesInput) (usecase.ListActivitiesOutput, error) {
		if in.TodoID != "none" {
			t.Errorf("todo id = %q", in.TodoID)
		}
		return usecase.ListActivitiesOutput{}, nil
	}}
	assertResponse(t, f.serve("GET", "/todos/none/activities", ""), http.StatusOK, `{"items":[]}`)
}
