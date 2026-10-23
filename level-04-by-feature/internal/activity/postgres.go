package activity

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"example.com/todo/level04/internal/platform/tx"
)

type activityRow struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	TodoID    string    `gorm:"type:uuid;not null;index"`
	Action    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:false"`
}

func (activityRow) TableName() string { return "activities" }

func toActivityRow(a Activity) activityRow {
	return activityRow{ID: a.ID, TodoID: a.TodoID, Action: a.Action, CreatedAt: a.CreatedAt}
}

// toActivity converts to UTC because pgx returns timestamptz in local time.
func (r activityRow) toActivity() Activity {
	return Activity{ID: r.ID, TodoID: r.TodoID, Action: r.Action, CreatedAt: r.CreatedAt.UTC()}
}

// MigratePostgres creates or updates this feature's table and its todo_id index.
func MigratePostgres(db *gorm.DB) error {
	if err := db.AutoMigrate(&activityRow{}); err != nil {
		return fmt.Errorf("migrate activities: %w", err)
	}
	return nil
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, a Activity) error {
	row := toActivityRow(a)
	if err := tx.Conn(ctx, r.db).Create(&row).Error; err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListByTodo(ctx context.Context, todoID string) ([]Activity, error) {
	var rows []activityRow
	if err := tx.Conn(ctx, r.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	activities := make([]Activity, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, row.toActivity())
	}
	return activities, nil
}
