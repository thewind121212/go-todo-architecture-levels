package usecase

import "context"

type ListActivitiesInput struct {
	TodoID string
}

type ListActivitiesOutput struct {
	Items []ActivityOutput
}

// ListActivities returns a todo's activities, oldest first. It does not check that the todo exists,
// because activities outlive deleted todos.
type ListActivities struct {
	activities ActivityRepository
}

func NewListActivities(activities ActivityRepository) *ListActivities {
	return &ListActivities{activities: activities}
}

func (uc *ListActivities) Execute(ctx context.Context, in ListActivitiesInput) (ListActivitiesOutput, error) {
	activities, err := uc.activities.ListByTodo(ctx, in.TodoID)
	if err != nil {
		return ListActivitiesOutput{}, err
	}
	items := make([]ActivityOutput, 0, len(activities))
	for _, a := range activities {
		items = append(items, newActivityOutput(a))
	}
	return ListActivitiesOutput{Items: items}, nil
}
