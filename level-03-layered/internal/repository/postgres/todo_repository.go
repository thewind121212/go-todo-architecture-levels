package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level03/internal/model"
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

func toTodoRow(t model.Todo) todoRow {
	return todoRow{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

// toModel converts to UTC because pgx returns timestamptz in the process's
// local time zone, while the API promises UTC.
func (r todoRow) toModel() model.Todo {
	return model.Todo{ID: r.ID, Title: r.Title, Completed: r.Completed, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
}

type TodoRepository struct {
	db *gorm.DB
}

func NewTodoRepository(db *gorm.DB) *TodoRepository {
	return &TodoRepository{db: db}
}

func (r *TodoRepository) Create(ctx context.Context, todo model.Todo) error {
	row := toTodoRow(todo)
	if err := conn(ctx, r.db).Create(&row).Error; err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) Get(ctx context.Context, id string) (model.Todo, error) {
	return findTodo(conn(ctx, r.db), id)
}

// GetForUpdate adds FOR UPDATE: the row stays locked until the transaction
// ends, so two concurrent PATCHes can't both see "not completed" and both log
// a "completed" activity.
func (r *TodoRepository) GetForUpdate(ctx context.Context, id string) (model.Todo, error) {
	return findTodo(conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]model.Todo, error) {
	var rows []todoRow
	err := conn(ctx, r.db).Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	todos := make([]model.Todo, 0, len(rows))
	for _, row := range rows {
		todos = append(todos, row.toModel())
	}
	return todos, nil
}

// Update writes the mutable columns. A map is used so that completed=false
// is written too (GORM skips zero values when updating from a struct).
func (r *TodoRepository) Update(ctx context.Context, todo model.Todo) error {
	res := conn(ctx, r.db).Model(&todoRow{}).Where("id = ?", todo.ID).Updates(map[string]any{
		"title":      todo.Title,
		"completed":  todo.Completed,
		"updated_at": todo.UpdatedAt,
	})
	if res.Error != nil {
		return fmt.Errorf("update todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res := conn(ctx, r.db).Where("id = ?", id).Delete(&todoRow{})
	if res.Error != nil {
		return fmt.Errorf("delete todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return model.ErrNotFound
	}
	return nil
}

// findTodo works on a plain or a locking query, so Get and GetForUpdate share it.
func findTodo(db *gorm.DB, id string) (model.Todo, error) {
	var row todoRow
	err := db.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Todo{}, model.ErrNotFound
	}
	if err != nil {
		return model.Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return row.toModel(), nil
}
