package activity

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level04/internal/platform/tx"
)

// MigratePostgres creates or updates the activities table and its todo_id index.
func MigratePostgres(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&Activity{}); err != nil {
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
	if err := tx.PostgresDB(ctx, r.db).Create(&a).Error; err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListByTodo(ctx context.Context, todoID string) ([]Activity, error) {
	activities := []Activity{}
	err := tx.PostgresDB(ctx, r.db).Where("todo_id = ?", todoID).Order("id ASC").Find(&activities).Error
	if err != nil {
		return nil, fmt.Errorf("select activities: %w", err)
	}
	// pgx returns timestamptz values in the local time zone.
	for i := range activities {
		activities[i].CreatedAt = activities[i].CreatedAt.UTC()
	}
	return activities, nil
}
