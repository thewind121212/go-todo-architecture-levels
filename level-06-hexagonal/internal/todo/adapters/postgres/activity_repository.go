package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// ActivityRepository implements ports.ActivityRepository.
type ActivityRepository struct {
	db *gorm.DB
}

var _ ports.ActivityRepository = (*ActivityRepository)(nil)

func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Append(ctx context.Context, activity *domain.Activity) error {
	if err := conn(ctx, r.db).Create(toActivityModel(activity)).Error; err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodo(ctx context.Context, todoID string) ([]*domain.Activity, error) {
	var models []activityModel
	// UUIDv7 ids are time-ordered, so id ASC means chronological.
	err := conn(ctx, r.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select activities: %w", err)
	}
	activities := make([]*domain.Activity, 0, len(models))
	for i := range models {
		activities = append(activities, toDomainActivity(&models[i]))
	}
	return activities, nil
}
