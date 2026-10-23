package activity

import (
	"context"
	"testing"
	"time"
)

func TestRecordAndList(t *testing.T) {
	svc := NewService(&fakeRepo{})
	ctx := context.Background()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, action := range []string{"created", "renamed"} {
		if err := svc.Record(ctx, todoID, action, at); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	got, err := svc.List(ctx, todoID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].Action != "created" || got[1].Action != "renamed" {
		t.Fatalf("List = %+v", got)
	}
	if got[0].ID == "" || got[0].TodoID != todoID || !got[0].CreatedAt.Equal(at) {
		t.Errorf("activity = %+v, want id, todo id and created_at = %v", got[0], at)
	}
}
