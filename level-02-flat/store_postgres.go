package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type postgresStore struct {
	db *gorm.DB
}

func newPostgresStore(dsn string) (*postgresStore, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.New(log.Default(), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // a 404 is not worth a log line
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.AutoMigrate(&Todo{}, &Activity{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &postgresStore{db: db}, nil
}

// AfterFind is a GORM hook. The pgx driver returns timestamptz in the
// machine's local zone, but the API promises UTC.
func (t *Todo) AfterFind(*gorm.DB) error {
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return nil
}

// AfterFind: see Todo.AfterFind.
func (a *Activity) AfterFind(*gorm.DB) error {
	a.CreatedAt = a.CreatedAt.UTC()
	return nil
}

// inTx is where every Postgres transaction starts. GORM commits when fn
// returns nil and rolls back when it returns an error.
func (s *postgresStore) inTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

func (s *postgresStore) Create(ctx context.Context, title string) (Todo, error) {
	todo, err := newTodo(title)
	if err != nil {
		return Todo{}, err
	}
	created := newActivity(todo.ID, ActionCreated, todo.CreatedAt)

	err = s.inTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(&todo).Error; err != nil {
			return fmt.Errorf("insert todo: %w", err)
		}
		if err := tx.Create(&created).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *postgresStore) List(ctx context.Context, limit, offset int) ([]Todo, error) {
	todos := []Todo{}
	err := s.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&todos).Error
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	return todos, nil
}

func (s *postgresStore) Get(ctx context.Context, id string) (Todo, error) {
	var todo Todo
	if err := s.db.WithContext(ctx).First(&todo, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Todo{}, ErrNotFound
		}
		return Todo{}, fmt.Errorf("get todo: %w", err)
	}
	return todo, nil
}

func (s *postgresStore) Update(ctx context.Context, id string, p todoPatch) (Todo, error) {
	p, err := validatePatch(p)
	if err != nil {
		return Todo{}, err
	}

	var todo Todo
	err = s.inTx(ctx, func(tx *gorm.DB) error {
		// FOR UPDATE locks the row, so two concurrent PATCHes cannot both
		// see "not completed" and both log a "completed" activity.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&todo, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get todo: %w", err)
		}

		activities := applyPatch(&todo, p, now())
		if len(activities) == 0 {
			return nil
		}
		if err := tx.Save(&todo).Error; err != nil {
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
	return todo, nil
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	deleted := newActivity(id, ActionDeleted, now())

	return s.inTx(ctx, func(tx *gorm.DB) error {
		res := tx.Delete(&Todo{}, "id = ?", id)
		if res.Error != nil {
			return fmt.Errorf("delete todo: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := tx.Create(&deleted).Error; err != nil {
			return fmt.Errorf("insert activity: %w", err)
		}
		return nil
	})
}

func (s *postgresStore) Activities(ctx context.Context, todoID string) ([]Activity, error) {
	activities := []Activity{}
	err := s.db.WithContext(ctx).Where("todo_id = ?", todoID).Order("id ASC").Find(&activities).Error
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	return activities, nil
}

func (s *postgresStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}
