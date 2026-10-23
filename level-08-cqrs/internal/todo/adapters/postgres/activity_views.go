package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level08/internal/todo/application/query"
)

// ActivityViews implements query.ActivityViews. Queries read it; the projector writes it
// inside the relay's transaction.
type ActivityViews struct {
	db *gorm.DB
}

var _ query.ActivityViews = (*ActivityViews)(nil)

func NewActivityViews(db *gorm.DB) *ActivityViews {
	return &ActivityViews{db: db}
}

func (v *ActivityViews) ListByTodo(ctx context.Context, todoID string) ([]*query.ActivityView, error) {
	var models []activityViewModel
	// UUIDv7 ids are time-ordered, so id ASC means chronological.
	err := conn(ctx, v.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select activity views: %w", err)
	}
	activities := make([]*query.ActivityView, 0, len(models))
	for i := range models {
		activities = append(activities, toActivityView(&models[i]))
	}
	return activities, nil
}

// Add is INSERT ... ON CONFLICT (id) DO NOTHING: the id is the event id, so a repeated
// event finds its activity already there and changes nothing.
func (v *ActivityViews) Add(ctx context.Context, activity *query.ActivityView) error {
	err := conn(ctx, v.db).Clauses(clause.OnConflict{DoNothing: true}).Create(toActivityViewModel(activity)).Error
	if err != nil {
		return fmt.Errorf("insert activity view: %w", err)
	}
	return nil
}
