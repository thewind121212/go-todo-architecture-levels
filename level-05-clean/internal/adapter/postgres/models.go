package postgres

import (
	"time"

	"github.com/google/uuid"

	"example.com/todo/level05/internal/entity"
)

// The persistence models carry the gorm tags, so the entities don't have to. The mappers below are
// the price of that separation, and also the place where storage details (like the time zone pgx
// returns) are normalized away.

type todoModel struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Title     string    `gorm:"not null"`
	Completed bool      `gorm:"not null;default:false"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:false"` // the app sets timestamps, not GORM
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:false"`
}

func (todoModel) TableName() string { return "todos" }

type activityModel struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	TodoID    string    `gorm:"type:uuid;not null;index"` // no foreign key: activities outlive todos
	Action    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:false"`
}

func (activityModel) TableName() string { return "activities" }

func toTodoModel(t entity.Todo) todoModel {
	return todoModel{ID: t.ID, Title: t.Title, Completed: t.Completed, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
}

func toTodoEntity(m todoModel) entity.Todo {
	return entity.Todo{
		ID:        m.ID,
		Title:     m.Title,
		Completed: m.Completed,
		CreatedAt: m.CreatedAt.UTC(),
		UpdatedAt: m.UpdatedAt.UTC(),
	}
}

func toActivityModel(a entity.Activity) activityModel {
	return activityModel{ID: a.ID, TodoID: a.TodoID, Action: string(a.Action), CreatedAt: a.CreatedAt}
}

func toActivityEntity(m activityModel) entity.Activity {
	return entity.Activity{ID: m.ID, TodoID: m.TodoID, Action: entity.Action(m.Action), CreatedAt: m.CreatedAt.UTC()}
}

// isUUID guards queries on uuid columns: Postgres rejects a malformed UUID with an error (which would
// also abort the transaction), while to the app such an ID simply matches nothing.
func isUUID(id string) bool {
	return uuid.Validate(id) == nil
}
