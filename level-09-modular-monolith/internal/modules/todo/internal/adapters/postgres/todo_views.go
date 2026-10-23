package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"example.com/todo/level09/internal/modules/todo/internal/application/query"
)

// TodoViews implements query.TodoViews. Queries read it; the projector writes it
// inside its own transaction.
type TodoViews struct {
	db *gorm.DB
}

var _ query.TodoViews = (*TodoViews)(nil)

func NewTodoViews(db *gorm.DB) *TodoViews {
	return &TodoViews{db: db}
}

func (v *TodoViews) Get(ctx context.Context, id string) (*query.TodoView, error) {
	var m todoViewModel
	err := conn(ctx, v.db).Where("id = ?", id).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, query.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select todo view: %w", err)
	}
	return toTodoView(&m), nil
}

func (v *TodoViews) List(ctx context.Context, limit, offset int) ([]*query.TodoView, error) {
	var models []todoViewModel
	// UUIDv7 ids are time-ordered, so id DESC means newest first.
	err := conn(ctx, v.db).Order("id DESC").Limit(limit).Offset(offset).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("select todo views: %w", err)
	}
	views := make([]*query.TodoView, 0, len(models))
	for i := range models {
		views = append(views, toTodoView(&models[i]))
	}
	return views, nil
}

// Upsert is INSERT ... ON CONFLICT (id) DO UPDATE: the row ends up equal to view either way.
func (v *TodoViews) Upsert(ctx context.Context, view *query.TodoView) error {
	err := conn(ctx, v.db).Clauses(clause.OnConflict{UpdateAll: true}).Create(toTodoViewModel(view)).Error
	if err != nil {
		return fmt.Errorf("upsert todo view: %w", err)
	}
	return nil
}

func (v *TodoViews) Delete(ctx context.Context, id string) error {
	if err := conn(ctx, v.db).Where("id = ?", id).Delete(&todoViewModel{}).Error; err != nil {
		return fmt.Errorf("delete todo view: %w", err)
	}
	return nil
}
