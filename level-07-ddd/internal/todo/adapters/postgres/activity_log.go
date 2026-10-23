package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level07/internal/todo/domain"
	"example.com/todo/level07/internal/todo/ports"
)

// ActivityLog implements ports.ActivityLog. The rows are written by TodoRepository.Save.
type ActivityLog struct {
	db *gorm.DB
}

var _ ports.ActivityLog = (*ActivityLog)(nil)

func NewActivityLog(db *gorm.DB) *ActivityLog {
	return &ActivityLog{db: db}
}

func (l *ActivityLog) ListByTodo(ctx context.Context, todoID domain.TodoID) ([]*domain.Activity, error) {
	var models []activityModel
	// UUIDv7 ids are time-ordered, so id ASC means chronological.
	err := l.db.WithContext(ctx).Where("todo_id = ?", todoID.String()).Order("id ASC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select activities: %w", err)
	}
	activities := make([]*domain.Activity, 0, len(models))
	for i := range models {
		activities = append(activities, toDomainActivity(&models[i]))
	}
	return activities, nil
}
