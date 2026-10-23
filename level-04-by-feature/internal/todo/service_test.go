package todo

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func newTestService() (*Service, *fakeRepo, *fakeRecorder) {
	repo := newFakeRepo()
	rec := &fakeRecorder{}
	return NewService(repo, fakeTx{}, rec), repo, rec
}

func TestCreate(t *testing.T) {
	svc, repo, rec := newTestService()

	got, err := svc.Create(context.Background(), "  buy milk  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Title != "buy milk" || got.Completed {
		t.Errorf("got %+v, want trimmed title and not completed", got)
	}
	if got.CreatedAt.IsZero() || got.CreatedAt != got.UpdatedAt {
		t.Errorf("timestamps not set: %+v", got)
	}
	if _, ok := repo.todos[got.ID]; !ok {
		t.Error("todo not stored")
	}
	want := []recorded{{todoID: got.ID, action: ActionCreated, inTx: true}}
	if !slices.Equal(rec.records, want) {
		t.Errorf("records = %+v, want %+v", rec.records, want)
	}
}

func TestCreateTitleRules(t *testing.T) {
	tests := []struct {
		title   string
		wantErr error
	}{
		{"   ", ErrInvalidTitle},
		{strings.Repeat("x", 201), ErrInvalidTitle},
		{strings.Repeat("x", 200), nil},
		{strings.Repeat("é", 200), nil}, // runes, not bytes
	}
	for _, tt := range tests {
		svc, _, _ := newTestService()
		_, err := svc.Create(context.Background(), tt.title)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("Create(%d runes) error = %v, want %v", len([]rune(tt.title)), err, tt.wantErr)
		}
	}
}

func TestCreateFailsWhenRecorderFails(t *testing.T) {
	svc, _, rec := newTestService()
	rec.err = errBoom

	if _, err := svc.Create(context.Background(), "x"); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}

func TestUpdateActions(t *testing.T) {
	tests := []struct {
		name  string
		patch Patch
		want  []string
	}{
		{"rename and complete", Patch{Title: new("new"), Completed: new(true)}, []string{ActionRenamed, ActionCompleted}},
		{"same title is no change", Patch{Title: new(" old ")}, nil},
		{"same completed is no change", Patch{Completed: new(false)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, rec := newTestService()
			created, _ := svc.Create(context.Background(), "old")
			rec.records = nil

			got, err := svc.Update(context.Background(), created.ID, tt.patch)
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if !slices.Equal(rec.actions(), tt.want) {
				t.Errorf("actions = %v, want %v", rec.actions(), tt.want)
			}
			if repo.todos[created.ID] != got {
				t.Errorf("stored %+v, returned %+v", repo.todos[created.ID], got)
			}
		})
	}
}

func TestUpdateReopen(t *testing.T) {
	svc, _, rec := newTestService()
	created, _ := svc.Create(context.Background(), "x")
	if _, err := svc.Update(context.Background(), created.ID, Patch{Completed: new(true)}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := svc.Update(context.Background(), created.ID, Patch{Completed: new(false)})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got.Completed {
		t.Error("still completed")
	}
	want := []string{ActionCreated, ActionCompleted, ActionReopened}
	if !slices.Equal(rec.actions(), want) {
		t.Errorf("actions = %v, want %v", rec.actions(), want)
	}
}

func TestUpdateErrors(t *testing.T) {
	svc, _, _ := newTestService()
	created, _ := svc.Create(context.Background(), "x")
	tests := []struct {
		name    string
		id      string
		patch   Patch
		wantErr error
	}{
		{"empty patch", created.ID, Patch{}, ErrEmptyUpdate},
		{"blank title", created.ID, Patch{Title: new(" ")}, ErrInvalidTitle},
		{"unknown id", "0190a0e4-0000-7000-8000-000000000000", Patch{Completed: new(true)}, ErrNotFound},
		{"malformed id", "nope", Patch{Completed: new(true)}, ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Update(context.Background(), tt.id, tt.patch); !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateFailsWhenRecorderFails(t *testing.T) {
	svc, _, rec := newTestService()
	created, _ := svc.Create(context.Background(), "x")
	rec.err = errBoom

	if _, err := svc.Update(context.Background(), created.ID, Patch{Completed: new(true)}); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}

func TestDelete(t *testing.T) {
	svc, repo, rec := newTestService()
	created, _ := svc.Create(context.Background(), "x")

	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := repo.todos[created.ID]; ok {
		t.Error("todo still stored")
	}
	last := rec.records[len(rec.records)-1]
	if last != (recorded{todoID: created.ID, action: ActionDeleted, inTx: true}) {
		t.Errorf("last record = %+v", last)
	}
	if err := svc.Delete(context.Background(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete error = %v, want %v", err, ErrNotFound)
	}
}

func TestDeleteFailsWhenRecorderFails(t *testing.T) {
	svc, _, rec := newTestService()
	created, _ := svc.Create(context.Background(), "x")
	rec.err = errBoom

	if err := svc.Delete(context.Background(), created.ID); !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want %v", err, errBoom)
	}
}

func TestList(t *testing.T) {
	svc, _, _ := newTestService()
	first, _ := svc.Create(context.Background(), "first")
	second, _ := svc.Create(context.Background(), "second")

	got, err := svc.List(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Errorf("List = %+v, want newest first", got)
	}

	for _, tt := range []struct {
		limit, offset int
		wantErr       error
	}{
		{0, 0, ErrInvalidLimit},
		{101, 0, ErrInvalidLimit},
		{20, -1, ErrInvalidOffset},
	} {
		if _, err := svc.List(context.Background(), tt.limit, tt.offset); !errors.Is(err, tt.wantErr) {
			t.Errorf("List(%d, %d) error = %v, want %v", tt.limit, tt.offset, err, tt.wantErr)
		}
	}
}
