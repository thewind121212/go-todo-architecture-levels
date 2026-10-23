package main

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Domain errors. handler.go maps them to HTTP status codes in writeError.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1-200 characters")
	ErrEmptyPatch   = errors.New("patch must set title or completed")
)

const maxTitleLen = 200

const (
	ActionCreated   = "created"
	ActionRenamed   = "renamed"
	ActionCompleted = "completed"
	ActionReopened  = "reopened"
	ActionDeleted   = "deleted"
)

// Todo carries json, gorm and bson tags at once: everything lives in one
// package, so one struct serves the API and both databases.
type Todo struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	Title     string    `json:"title" gorm:"not null" bson:"title"`
	Completed bool      `json:"completed" gorm:"not null;default:false" bson:"completed"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;autoUpdateTime:false" bson:"updated_at"`
}

// Activity is an append-only audit entry. TodoID has no foreign key so the
// history outlives a deleted todo.
type Activity struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	TodoID    string    `json:"todo_id" gorm:"type:uuid;not null;index" bson:"todo_id"`
	Action    string    `json:"action" gorm:"not null" bson:"action"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
}

// todoPatch is the PATCH body. A nil field means "leave it unchanged".
type todoPatch struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

// now is truncated to milliseconds because Mongo stores no finer precision;
// this way both stores round-trip identical values.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// newID panics instead of returning an error: uuid.NewV7 can only fail if
// crypto/rand fails, and Go treats that as fatal anyway.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

func validatePatch(p todoPatch) (todoPatch, error) {
	if p.Title == nil && p.Completed == nil {
		return p, ErrEmptyPatch
	}
	if p.Title != nil {
		title, err := validateTitle(*p.Title)
		if err != nil {
			return p, err
		}
		p.Title = &title
	}
	return p, nil
}

func newTodo(title string) (Todo, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	t := now()
	return Todo{ID: newID(), Title: title, CreatedAt: t, UpdatedAt: t}, nil
}

func newActivity(todoID, action string, at time.Time) Activity {
	return Activity{ID: newID(), TodoID: todoID, Action: action, CreatedAt: at}
}

// applyPatch changes t in place and returns one activity per real change,
// renamed before completed/reopened. No change means no activities.
// Both stores call it inside their transaction, after reading the todo.
func applyPatch(t *Todo, p todoPatch, at time.Time) []Activity {
	var activities []Activity
	if p.Title != nil && *p.Title != t.Title {
		t.Title = *p.Title
		activities = append(activities, newActivity(t.ID, ActionRenamed, at))
	}
	if p.Completed != nil && *p.Completed != t.Completed {
		t.Completed = *p.Completed
		action := ActionReopened
		if t.Completed {
			action = ActionCompleted
		}
		activities = append(activities, newActivity(t.ID, action, at))
	}
	if len(activities) > 0 {
		t.UpdatedAt = at
	}
	return activities
}
