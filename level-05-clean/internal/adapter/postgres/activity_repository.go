package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level05/internal/entity"
)

// ActivityRepository implements usecase.ActivityRepository.
type ActivityRepository struct {
	db *gorm.DB
}

func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Create(ctx context.Context, activity entity.Activity) error {
	m := toActivityModel(activity)
	if err := conn(ctx, r.db).Create(&m).Error; err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodoID(ctx context.Context, todoID string) ([]entity.Activity, error) {
	if !isUUID(todoID) {
		return []entity.Activity{}, nil
	}
	var models []activityModel
	// UUIDv7 IDs are time-ordered, so ID ASC is chronological.
	if err := conn(ctx, r.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := make([]entity.Activity, 0, len(models))
	for _, m := range models {
		activities = append(activities, toActivityEntity(m))
	}
	return activities, nil
}
