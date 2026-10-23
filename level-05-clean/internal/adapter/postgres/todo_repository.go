package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level05/internal/entity"
)

// TodoRepository implements usecase.TodoRepository.
type TodoRepository struct {
	db *gorm.DB
}

func NewTodoRepository(db *gorm.DB) *TodoRepository {
	return &TodoRepository{db: db}
}

func (r *TodoRepository) Create(ctx context.Context, todo entity.Todo) error {
	m := toTodoModel(todo)
	if err := conn(ctx, r.db).Create(&m).Error; err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) Get(ctx context.Context, id string) (entity.Todo, error) {
	return get(conn(ctx, r.db), id)
}

// GetForUpdate locks the row (SELECT ... FOR UPDATE) until the transaction ends, so two concurrent
// PATCHes can't both see completed=false and both log "completed".
func (r *TodoRepository) GetForUpdate(ctx context.Context, id string) (entity.Todo, error) {
	return get(conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
}

func get(db *gorm.DB, id string) (entity.Todo, error) {
	var m todoModel
	err := db.First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.Todo{}, entity.ErrNotFound
	}
	if err != nil {
		return entity.Todo{}, fmt.Errorf("select todo: %w", err)
	}
	return toTodoEntity(m), nil
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]entity.Todo, error) {
	var models []todoModel
	// UUIDv7 IDs are time-ordered, so ID DESC is newest first.
	if err := conn(ctx, r.db).Order("id DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	todos := make([]entity.Todo, 0, len(models))
	for _, m := range models {
		todos = append(todos, toTodoEntity(m))
	}
	return todos, nil
}

func (r *TodoRepository) Update(ctx context.Context, todo entity.Todo) error {
	m := toTodoModel(todo)
	// Select names the columns so GORM also writes zero values such as completed=false.
	res := conn(ctx, r.db).Model(&m).Select("title", "completed", "updated_at").Updates(&m)
	if res.Error != nil {
		return fmt.Errorf("update todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res := conn(ctx, r.db).Delete(&todoModel{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return entity.ErrNotFound
	}
	return nil
}
