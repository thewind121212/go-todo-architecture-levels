// Package postgres is a driven adapter: it implements domain.TodoRepository (write side),
// query.TodoViews and query.ActivityViews (read side), and outbox.Store and outbox.TxManager
// (relay) with GORM. The schema itself comes from SQL migrations.
package postgres

import "time"

// todoModel is the row shape of the todos table. It is kept apart from
// domain.Todo so database details never leak into the domain.
type todoModel struct {
	ID        string
	Title     string
	Completed bool
	// Timestamps come from the domain; stop GORM from overwriting them.
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:false"`
	Version   int
}

func (todoModel) TableName() string { return "todos" }

// outboxModel is the row shape of the outbox table: one serialized domain event.
type outboxModel struct {
	ID          string
	TodoID      string
	Name        string
	Payload     []byte // jsonb
	OccurredAt  time.Time
	ProcessedAt *time.Time // NULL until the relay has handled it
}

func (outboxModel) TableName() string { return "outbox" }

// todoViewModel is the row shape of the todo_views read model.
type todoViewModel struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:false"`
}

func (todoViewModel) TableName() string { return "todo_views" }

// activityViewModel is the row shape of the activity_views read model.
type activityViewModel struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
}

func (activityViewModel) TableName() string { return "activity_views" }
