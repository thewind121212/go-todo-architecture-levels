// Package postgres is a driven adapter: it implements the repository and
// transaction ports with GORM. The schema itself comes from SQL migrations.
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
}

func (todoModel) TableName() string { return "todos" }

// activityModel is the row shape of the activities table.
type activityModel struct {
	ID        string
	TodoID    string
	Action    string
	CreatedAt time.Time `gorm:"autoCreateTime:false"`
}

func (activityModel) TableName() string { return "activities" }
