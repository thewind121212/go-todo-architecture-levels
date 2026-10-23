// Package postgres is a driven adapter: it implements the activity module's ActivityLog and
// TxManager with GORM. Its one table lives in the module's own schema, "activity".
package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level09/internal/modules/activity/internal/application"
)

// ActivityLog implements application.ActivityLog.
type ActivityLog struct {
	db *gorm.DB
}

var _ application.ActivityLog = (*ActivityLog)(nil)

func NewActivityLog(db *gorm.DB) *ActivityLog {
	return &ActivityLog{db: db}
}

func (l *ActivityLog) ListByTodo(ctx context.Context, todoID string) ([]*application.Activity, error) {
	var models []activityModel
	// UUIDv7 ids are time-ordered, so id ASC means chronological.
	err := conn(ctx, l.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select activities: %w", err)
	}
	activities := make([]*application.Activity, 0, len(models))
	for i := range models {
		activities = append(activities, toActivity(&models[i]))
	}
	return activities, nil
}

// Add is INSERT ... ON CONFLICT (id) DO NOTHING: the id is the event id, so a repeated
// event finds its activity already there and changes nothing.
func (l *ActivityLog) Add(ctx context.Context, activity *application.Activity) error {
	err := conn(ctx, l.db).Clauses(clause.OnConflict{DoNothing: true}).Create(toActivityModel(activity)).Error
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}
