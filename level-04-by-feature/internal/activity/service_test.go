package activity

import (
	"context"
	"testing"
)

func TestRecordAndList(t *testing.T) {
	svc := NewService(&fakeRepo{})
	ctx := context.Background()
	for _, action := range []string{"created", "renamed"} {
		if err := svc.Record(ctx, todoID, action); err != nil {
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
	if got[0].ID == "" || got[0].TodoID != todoID || got[0].CreatedAt.IsZero() {
		t.Errorf("activity not filled in: %+v", got[0])
	}
}

func TestListMalformedIDIsEmpty(t *testing.T) {
	got, err := NewService(&fakeRepo{}).List(context.Background(), "not-a-uuid")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("List = %v, %v; want empty, non-nil", got, err)
	}
}
