package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"example.com/todo/level03/internal/model"
	"example.com/todo/level03/internal/repository/fake"
	"example.com/todo/level03/internal/service"
)

type testDeps struct {
	svc        *service.TodoService
	tx         *fake.TxManager
	activities *fake.ActivityRepository
}

func newTestService() testDeps {
	tx := &fake.TxManager{}
	activities := &fake.ActivityRepository{}
	return testDeps{
		svc:        service.NewTodoService(tx, fake.NewTodoRepository(), activities),
		tx:         tx,
		activities: activities,
	}
}

func actionsOf(t *testing.T, d testDeps, todoID string) []model.Action {
	t.Helper()
	list, err := d.svc.ListActivities(t.Context(), todoID)
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	var actions []model.Action
	for _, a := range list {
		actions = append(actions, a.Action)
	}
	return actions
}

func TestCreate(t *testing.T) {
	d := newTestService()

	todo, err := d.svc.Create(t.Context(), "  buy milk ")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if todo.Title != "buy milk" {
		t.Errorf("title = %q, want %q", todo.Title, "buy milk")
	}
	if got := actionsOf(t, d, todo.ID); !slices.Equal(got, []model.Action{model.ActionCreated}) {
		t.Errorf("activities = %v, want [created]", got)
	}
	if d.tx.Commits != 1 {
		t.Errorf("commits = %d, want 1", d.tx.Commits)
	}
}

func TestCreateInvalidTitleStartsNoTransaction(t *testing.T) {
	d := newTestService()

	_, err := d.svc.Create(t.Context(), " ")
	if !errors.Is(err, model.ErrInvalidTitle) {
		t.Fatalf("err = %v, want ErrInvalidTitle", err)
	}
	if d.tx.Commits+d.tx.Rollbacks != 0 {
		t.Errorf("transactions = %d, want 0", d.tx.Commits+d.tx.Rollbacks)
	}
}

func TestUpdateRecordsOneActivityPerChange(t *testing.T) {
	d := newTestService()
	todo, err := d.svc.Create(t.Context(), "old")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := d.svc.Update(t.Context(), todo.ID, model.Patch{Title: new("new"), Completed: new(true)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Title != "new" || !got.Completed {
		t.Errorf("got %+v, want title new and completed", got)
	}
	want := []model.Action{model.ActionCreated, model.ActionRenamed, model.ActionCompleted}
	if acts := actionsOf(t, d, todo.ID); !slices.Equal(acts, want) {
		t.Errorf("activities = %v, want %v", acts, want)
	}
}

func TestUpdateWithoutChangeWritesNothing(t *testing.T) {
	d := newTestService()
	todo, err := d.svc.Create(t.Context(), "same")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := d.svc.Update(t.Context(), todo.ID, model.Patch{Title: new("same"), Completed: new(false)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got != todo {
		t.Errorf("got %+v, want unchanged %+v", got, todo)
	}
	if acts := actionsOf(t, d, todo.ID); len(acts) != 1 {
		t.Errorf("activities = %v, want only [created]", acts)
	}
}

func TestUpdateFailsWhenActivityInsertFails(t *testing.T) {
	d := newTestService()
	todo, err := d.svc.Create(t.Context(), "title")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	errInsert := errors.New("insert failed")
	d.activities.Err = errInsert

	_, err = d.svc.Update(t.Context(), todo.ID, model.Patch{Completed: new(true)})
	if !errors.Is(err, errInsert) {
		t.Fatalf("err = %v, want %v", err, errInsert)
	}
	if d.tx.Rollbacks != 1 {
		t.Errorf("rollbacks = %d, want 1: the transaction must report the failure", d.tx.Rollbacks)
	}
}

func TestNotFound(t *testing.T) {
	unknown := uuid.NewString()
	tests := []struct {
		name string
		id   string
	}{
		{name: "unknown id", id: unknown},
		{name: "malformed id", id: "not-a-uuid"},
		{name: "non-canonical id", id: "{" + unknown + "}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestService()
			if _, err := d.svc.Get(t.Context(), tt.id); !errors.Is(err, model.ErrNotFound) {
				t.Errorf("get: err = %v, want ErrNotFound", err)
			}
			if _, err := d.svc.Update(t.Context(), tt.id, model.Patch{Completed: new(true)}); !errors.Is(err, model.ErrNotFound) {
				t.Errorf("update: err = %v, want ErrNotFound", err)
			}
			if err := d.svc.Delete(t.Context(), tt.id); !errors.Is(err, model.ErrNotFound) {
				t.Errorf("delete: err = %v, want ErrNotFound", err)
			}
			if acts := actionsOf(t, d, tt.id); len(acts) != 0 {
				t.Errorf("activities = %v, want none", acts)
			}
		})
	}
}

func TestDeleteKeepsActivities(t *testing.T) {
	d := newTestService()
	todo, err := d.svc.Create(t.Context(), "title")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := d.svc.Delete(t.Context(), todo.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := d.svc.Get(t.Context(), todo.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("get after delete: err = %v, want ErrNotFound", err)
	}
	want := []model.Action{model.ActionCreated, model.ActionDeleted}
	if acts := actionsOf(t, d, todo.ID); !slices.Equal(acts, want) {
		t.Errorf("activities = %v, want %v", acts, want)
	}
}
