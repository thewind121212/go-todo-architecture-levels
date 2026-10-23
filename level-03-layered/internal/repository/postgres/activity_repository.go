package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"example.com/todo/level03/internal/model"
)

type activityRow struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	TodoID    string    `gorm:"type:uuid;not null;index"`
	Action    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:false"`
}

func (activityRow) TableName() string { return "activities" }

type ActivityRepository struct {
	db *gorm.DB
}

func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Create(ctx context.Context, a model.Activity) error {
	row := activityRow{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
	if err := conn(ctx, r.db).Create(&row).Error; err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *ActivityRepository) ListByTodo(ctx context.Context, todoID string) ([]model.Activity, error) {
	var rows []activityRow
	if err := conn(ctx, r.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := make([]model.Activity, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, model.Activity{
			ID: row.ID, TodoID: row.TodoID, Action: model.Action(row.Action), CreatedAt: row.CreatedAt.UTC(),
		})
	}
	return activities, nil
}
