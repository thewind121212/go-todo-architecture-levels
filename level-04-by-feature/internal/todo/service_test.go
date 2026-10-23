package todo

import (
	"context"
	"errors"
	"slices"
	"testing"
)

const unknownID = "0190a0e4-0000-7000-8000-000000000000"

type testService struct {
	*Service
	tx   *fakeTx
	repo *fakeRepo
	rec  *fakeRecorder
}

func newTestService() testService {
	s := testService{tx: &fakeTx{}, repo: newFakeRepo(), rec: &fakeRecorder{}}
	s.Service = NewService(s.tx, s.repo, s.rec)
	return s
}

func TestCreateRecordsInSameTx(t *testing.T) {
	s := newTestService()

	got, err := s.Create(context.Background(), "  buy milk  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.repo.todos[got.ID] != got {
		t.Errorf("stored %+v, returned %+v", s.repo.todos[got.ID], got)
	}
	want := []recorded{{todoID: got.ID, action: "created", at: got.CreatedAt, inTx: true}}
	if !slices.Equal(s.rec.records, want) {
		t.Errorf("records = %+v, want %+v", s.rec.records, want)
	}
}

func TestCreateInvalidTitleSkipsTx(t *testing.T) {
	s := newTestService()

	if _, err := s.Create(context.Background(), " "); !errors.Is(err, ErrInvalidTitle) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidTitle)
	}
	if s.tx.calls != 0 {
		t.Errorf("WithinTx called %d times, want 0", s.tx.calls)
	}
}

func TestUpdateRecordsEachChangeAtUpdatedAt(t *testing.T) {
	s := newTestService()
	todo, _ := s.Create(context.Background(), "old")
	s.rec.records = nil

	got, err := s.Update(context.Background(), todo.ID, Patch{Title: new(" new "), Completed: new(true)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Title != "new" || !got.Completed || s.repo.todos[todo.ID] != got {
		t.Errorf("got %+v, stored %+v", got, s.repo.todos[todo.ID])
	}
	want := []recorded{
		{todoID: todo.ID, action: "renamed", at: got.UpdatedAt, inTx: true},
		{todoID: todo.ID, action: "completed", at: got.UpdatedAt, inTx: true},
	}
	if !slices.Equal(s.rec.records, want) {
		t.Errorf("records = %+v, want %+v", s.rec.records, want)
	}
}

func TestUpdateWithoutChangeRecordsNothing(t *testing.T) {
	s := newTestService()
	todo, _ := s.Create(context.Background(), "same")
	s.rec.records = nil

	got, err := s.Update(context.Background(), todo.ID, Patch{Title: new("same"), Completed: new(false)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got != todo || len(s.rec.records) != 0 {
		t.Errorf("got %+v and records %+v, want unchanged todo and no records", got, s.rec.records)
	}
}

func TestUpdateReopen(t *testing.T) {
	s := newTestService()
	todo, _ := s.Create(context.Background(), "x")
	for _, completed := range []bool{true, false} {
		if _, err := s.Update(context.Background(), todo.ID, Patch{Completed: new(completed)}); err != nil {
			t.Fatalf("Update(completed=%v): %v", completed, err)
		}
	}
	if want := []string{"created", "completed", "reopened"}; !slices.Equal(s.rec.actions(), want) {
		t.Errorf("actions = %v, want %v", s.rec.actions(), want)
	}
}

func TestUpdateValidatesBeforeTx(t *testing.T) {
	tests := []struct {
		name    string
		patch   Patch
		wantErr error
	}{
		{"empty patch", Patch{}, ErrEmptyPatch},
		{"blank title", Patch{Title: new(" ")}, ErrInvalidTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService()
			// The id is unknown, yet the answer is the validation error: input is
			// checked before any database call.
			if _, err := s.Update(context.Background(), unknownID, tt.patch); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if s.tx.calls != 0 {
				t.Errorf("WithinTx called %d times, want 0", s.tx.calls)
			}
		})
	}
}

func TestUpdateUnknownID(t *testing.T) {
	s := newTestService()

	if _, err := s.Update(context.Background(), unknownID, Patch{Completed: new(true)}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want %v", err, ErrNotFound)
	}
}

func TestDeleteRecordsInSameTx(t *testing.T) {
	s := newTestService()
	todo, _ := s.Create(context.Background(), "x")

	if err := s.Delete(context.Background(), todo.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := s.repo.todos[todo.ID]; ok {
		t.Error("todo still stored")
	}
	if last := s.rec.records[len(s.rec.records)-1]; last.action != "deleted" || !last.inTx {
		t.Errorf("last record = %+v, want deleted inside the tx", last)
	}
	if err := s.Delete(context.Background(), todo.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete err = %v, want %v", err, ErrNotFound)
	}
}

// Every write fails when the activity feature fails: the error reaches the
// caller, so WithinTx rolls back.
func TestWritesFailWhenRecorderFails(t *testing.T) {
	s := newTestService()
	todo, _ := s.Create(context.Background(), "x")
	s.rec.err = errBoom

	if _, err := s.Create(context.Background(), "y"); !errors.Is(err, errBoom) {
		t.Errorf("Create err = %v, want %v", err, errBoom)
	}
	if _, err := s.Update(context.Background(), todo.ID, Patch{Completed: new(true)}); !errors.Is(err, errBoom) {
		t.Errorf("Update err = %v, want %v", err, errBoom)
	}
	if err := s.Delete(context.Background(), todo.ID); !errors.Is(err, errBoom) {
		t.Errorf("Delete err = %v, want %v", err, errBoom)
	}
}
