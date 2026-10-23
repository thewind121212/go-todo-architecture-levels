package todo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level04/internal/platform/tx"
)

// todoRow is the table shape. The app sets the timestamps itself, so GORM's
// automatic created/updated time tracking is switched off.
type todoRow struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Title     string    `gorm:"not null"`
	Completed bool      `gorm:"not null;default:false"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:false"`
}

func (todoRow) TableName() string { return "todos" }

func toTodoRow(t Todo) todoRow {
	return todoRow{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

// toTodo converts to UTC because pgx returns timestamptz in local time.
func (r todoRow) toTodo() Todo {
	return Todo{ID: r.ID, Title: r.Title, Completed: r.Completed, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
}

// MigratePostgres creates or updates this feature's table.
func MigratePostgres(db *gorm.DB) error {
	if err := db.AutoMigrate(&todoRow{}); err != nil {
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

func (r *PostgresRepository) Create(ctx context.Context, todo Todo) error {
	row := toTodoRow(todo)
	if err := tx.Conn(ctx, r.db).Create(&row).Error; err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (Todo, error) {
	return take(tx.Conn(ctx, r.db), id)
}

// GetForUpdate locks the row (SELECT ... FOR UPDATE) until the transaction
// ends, so two concurrent PATCHes cannot both log "completed".
func (r *PostgresRepository) GetForUpdate(ctx context.Context, id string) (Todo, error) {
	return take(tx.Conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
}

func take(db *gorm.DB, id string) (Todo, error) {
	var row todoRow
	err := db.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return row.toTodo(), nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	var rows []todoRow
	err := tx.Conn(ctx, r.db).Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	todos := make([]Todo, 0, len(rows))
	for _, row := range rows {
		todos = append(todos, row.toTodo())
	}
	return todos, nil
}

// Update writes the mutable columns. A map is used so that completed=false
// is written too (GORM skips zero values when updating from a struct).
func (r *PostgresRepository) Update(ctx context.Context, todo Todo) error {
	res := tx.Conn(ctx, r.db).Model(&todoRow{}).Where("id = ?", todo.ID).Updates(map[string]any{
		"title":      todo.Title,
		"completed":  todo.Completed,
		"updated_at": todo.UpdatedAt,
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
	res := tx.Conn(ctx, r.db).Where("id = ?", id).Delete(&todoRow{})
	if res.Error != nil {
		return fmt.Errorf("delete todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
