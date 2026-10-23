// store_postgres.go: the Postgres store (GORM). Each write method owns its transaction.

package main

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type postgresStore struct {
	db *gorm.DB
}

func newPostgresStore(ctx context.Context, dsn string) (*postgresStore, error) {
	// Silent: errors are returned to the caller, and the HTTP layer logs the ones that matter.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&Todo{}, &Activity{}); err != nil {
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return &postgresStore{db: db}, nil
}

// inTx runs fn in one database transaction: commit if fn returns nil, roll back otherwise.
func (s *postgresStore) inTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

func (s *postgresStore) Create(ctx context.Context, t Todo) error {
	return s.inTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		a := newActivity(t.ID, ActionCreated, t.CreatedAt)
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *postgresStore) Get(ctx context.Context, id string) (Todo, error) {
	return findTodo(s.db.WithContext(ctx), id)
}

func (s *postgresStore) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	todos := []Todo{}
	err := s.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&todos).Error
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	for i := range todos {
		todos[i] = todoInUTC(todos[i])
	}
	return todos, nil
}

func (s *postgresStore) Update(ctx context.Context, id string, p Patch) (Todo, error) {
	var t Todo
	err := s.inTx(ctx, func(tx *gorm.DB) error {
		// FOR UPDATE locks the row until commit, so two concurrent PATCHes
		// can't both see "not completed" and both log a "completed" activity.
		var err error
		t, err = findTodo(tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}), id)
		if err != nil {
			return err
		}
		activities := applyPatch(&t, p, nowUTC())
		if len(activities) == 0 {
			return nil
		}
		if err := tx.Save(&t).Error; err != nil {
			return fmt.Errorf("update todo: %w", err)
		}
		if err := tx.Create(&activities).Error; err != nil {
			return fmt.Errorf("insert activities: %w", err)
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return t, nil
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&Todo{})
		if res.Error != nil {
			return fmt.Errorf("delete todo: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		a := newActivity(id, ActionDeleted, nowUTC())
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *postgresStore) ListActivities(ctx context.Context, todoID string) ([]Activity, error) {
	activities := []Activity{}
	err := s.db.WithContext(ctx).Where("todo_id = ?", todoID).Order("id ASC").Find(&activities).Error
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	for i := range activities {
		activities[i].CreatedAt = activities[i].CreatedAt.UTC()
	}
	return activities, nil
}

func (s *postgresStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

// findTodo works on a plain or a transactional *gorm.DB, so Get and Update share it.
func findTodo(db *gorm.DB, id string) (Todo, error) {
	var t Todo
	err := db.First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Todo{}, ErrNotFound
	}
	if err != nil {
		return Todo{}, fmt.Errorf("find todo: %w", err)
	}
	return todoInUTC(t), nil
}

// todoInUTC is needed because pgx returns timestamps in the process's local
// time zone, while the API promises UTC.
func todoInUTC(t Todo) Todo {
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t
}
