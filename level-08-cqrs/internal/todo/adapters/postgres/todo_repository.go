package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"example.com/todo/level08/internal/todo/domain"
)

// TodoRepository implements domain.TodoRepository.
type TodoRepository struct {
	db *gorm.DB
}

var _ domain.TodoRepository = (*TodoRepository)(nil)

func NewTodoRepository(db *gorm.DB) *TodoRepository {
	return &TodoRepository{db: db}
}

func (r *TodoRepository) Get(ctx context.Context, id domain.TodoID) (*domain.Todo, error) {
	var m todoModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select todo: %w", err)
	}
	return toDomainTodo(&m), nil
}

// Save is the write side's transaction. The aggregate is the consistency boundary, so it is
// also the transaction boundary: its new state and its events are committed together or
// not at all. The events go to the outbox, not to a message bus or straight into the read
// models: the outbox is a table in this same database, so one transaction covers it.
// GORM commits when the func returns nil and rolls back when it returns an error.
func (r *TodoRepository) Save(ctx context.Context, todo *domain.Todo) error {
	events := todo.PullEvents()
	if len(events) == 0 {
		return nil // nothing changed: no write, no event
	}
	messages, err := toOutboxModels(events)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveState(tx, todo); err != nil {
			return err
		}
		if err := tx.Create(&messages).Error; err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
		return nil
	})
}

// saveState inserts a never-saved todo (version 0) as version 1, or updates or deletes
// the stored row, writing version Version()+1.
func saveState(tx *gorm.DB, todo *domain.Todo) error {
	m := toTodoModel(todo)
	if todo.Version() == 0 {
		if err := tx.Create(m).Error; err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		return nil
	}

	// Optimistic lock: only overwrite the row we loaded; if someone saved since, nothing matches.
	var res *gorm.DB
	if todo.Deleted() {
		res = tx.Where("version = ?", todo.Version()).Delete(m)
	} else {
		// Select lists the columns explicitly so a false "completed" is written too.
		res = tx.Model(m).Where("version = ?", todo.Version()).
			Select("title", "completed", "updated_at", "version").Updates(m)
	}
	if res.Error != nil {
		return fmt.Errorf("save todo: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrConflict
	}
	return nil
}
