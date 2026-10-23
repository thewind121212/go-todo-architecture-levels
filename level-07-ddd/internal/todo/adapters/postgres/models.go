// Package postgres is a driven adapter: it implements domain.TodoRepository and
// ports.ActivityLog with GORM. The schema itself comes from SQL migrations.
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

// activityModel is the row shape of the activities table: one stored domain event.
type activityModel struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
}

func (activityModel) TableName() string { return "activities" }
