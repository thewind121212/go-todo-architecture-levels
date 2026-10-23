package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level06/internal/todo/domain"
	"example.com/todo/level06/internal/todo/ports"
)

// TodoRepository implements ports.TodoRepository.
type TodoRepository struct {
	db *gorm.DB
}

var _ ports.TodoRepository = (*TodoRepository)(nil)

func NewTodoRepository(db *gorm.DB) *TodoRepository {
	return &TodoRepository{db: db}
}

func (r *TodoRepository) Create(ctx context.Context, todo *domain.Todo) error {
	if err := conn(ctx, r.db).Create(toTodoModel(todo)).Error; err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *TodoRepository) Get(ctx context.Context, id string) (*domain.Todo, error) {
	return r.get(conn(ctx, r.db), id)
}

// GetForUpdate adds SELECT ... FOR UPDATE: two concurrent PATCHes on one todo queue up
// instead of both reading the old row and both logging "completed".
func (r *TodoRepository) GetForUpdate(ctx context.Context, id string) (*domain.Todo, error) {
	return r.get(conn(ctx, r.db).Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
}

func (r *TodoRepository) get(db *gorm.DB, id string) (*domain.Todo, error) {
	var m todoModel
	err := db.Where("id = ?", id).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select todo: %w", err)
	}
	return toDomainTodo(&m), nil
}

func (r *TodoRepository) List(ctx context.Context, limit, offset int) ([]*domain.Todo, error) {
	var models []todoModel
	// UUIDv7 ids are time-ordered, so id DESC means newest first.
	err := conn(ctx, r.db).Order("id DESC").Limit(limit).Offset(offset).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select todos: %w", err)
	}
	todos := make([]*domain.Todo, 0, len(models))
	for i := range models {
		todos = append(todos, toDomainTodo(&models[i]))
	}
	return todos, nil
}

func (r *TodoRepository) Update(ctx context.Context, todo *domain.Todo) error {
	m := toTodoModel(todo)
	// Select lists the columns explicitly so a false "completed" is written too.
	res := conn(ctx, r.db).Model(m).Select("title", "completed", "updated_at").Updates(m)
	if res.Error != nil {
		return fmt.Errorf("update todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *TodoRepository) Delete(ctx context.Context, id string) error {
	res := conn(ctx, r.db).Where("id = ?", id).Delete(&todoModel{})
	if res.Error != nil {
		return fmt.Errorf("delete todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
