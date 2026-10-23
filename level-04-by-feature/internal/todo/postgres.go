package todo

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level04/internal/platform/tx"
)

// MigratePostgres creates or updates the todos table.
func MigratePostgres(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&Todo{}); err != nil {
		return fmt.Errorf("migrate todos: %w", err)
	}
	return nil
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, t Todo) error {
	if err := tx.PostgresDB(ctx, r.db).Create(&t).Error; err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (Todo, error) {
	var t Todo
	err := tx.PostgresDB(ctx, r.db).Take(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("select todo: %w", err)
	}
	return inUTC(t), nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	todos := []Todo{}
	err := tx.PostgresDB(ctx, r.db).Order("id DESC").Limit(limit).Offset(offset).Find(&todos).Error
	if err != nil {
		return nil, fmt.Errorf("select todos: %w", err)
	}
	for i := range todos {
		todos[i] = inUTC(todos[i])
	}
	return todos, nil
}

func (r *PostgresRepository) Update(ctx context.Context, t Todo) error {
	// UpdateColumns writes exactly these values; Updates would overwrite
	// updated_at with GORM's own (untruncated) clock.
	res := tx.PostgresDB(ctx, r.db).Model(&Todo{}).Where("id = ?", t.ID).UpdateColumns(map[string]any{
		"title":      t.Title,
		"completed":  t.Completed,
		"updated_at": t.UpdatedAt,
	})
	if res.Error != nil {
		return fmt.Errorf("update todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	res := tx.PostgresDB(ctx, r.db).Delete(&Todo{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// inUTC undoes pgx returning timestamptz values in the local time zone.
func inUTC(t Todo) Todo {
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t
}
