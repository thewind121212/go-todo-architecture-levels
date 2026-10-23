// Package postgres is a driven adapter: it implements domain.TodoRepository (write side),
// query.TodoViews (read side), projection.TxManager (projector) and outbox.Store (relay)
// with GORM. Every table lives in the todo module's own schema, "todo", which the module's
// SQL migrations create.
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

func (todoModel) TableName() string { return "todo.todos" }

// outboxModel is the row shape of the outbox table: one serialized domain event.
type outboxModel struct {
	ID          string
	TodoID      string
	Name        string
	Payload     []byte // jsonb
	OccurredAt  time.Time
	ProcessedAt *time.Time // NULL until the relay has handled it
}

func (outboxModel) TableName() string { return "todo.outbox" }

// todoViewModel is the row shape of the todo_views read model.
type todoViewModel struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:false"`
}

func (todoViewModel) TableName() string { return "todo.todo_views" }
