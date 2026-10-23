package usecase_test

import (
	"context"
	"testing"

	"example.com/todo/level05/internal/entity"
	"example.com/todo/level05/internal/usecase"
)

func TestListActivities(t *testing.T) {
	activities := &fakeActivities{items: []entity.Activity{
		mustNewActivity("a", entity.ActionCreated),
		mustNewActivity("b", entity.ActionCreated),
		mustNewActivity("a", entity.ActionDeleted),
	}}
	uc := usecase.NewListActivities(activities)

	out, err := uc.Execute(context.Background(), usecase.ListActivitiesInput{TodoID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[0].Action != "created" || out.Items[1].Action != "deleted" {
		t.Errorf("unexpected items: %+v", out.Items)
	}

	out, err = uc.Execute(context.Background(), usecase.ListActivitiesInput{TodoID: "none"})
	if err != nil || out.Items == nil || len(out.Items) != 0 {
		t.Errorf("items = %#v, err = %v; want empty slice", out.Items, err)
	}
}
