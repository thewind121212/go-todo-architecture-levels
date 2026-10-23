// todo.go: the domain. Types, errors, and the rules both stores share.

package main

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Todo carries JSON, GORM and BSON tags at once: one type serves every layer.
// The created/updated times are set by the app, so GORM's auto-timestamps are off.
type Todo struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	Title     string    `json:"title" gorm:"not null" bson:"title"`
	Completed bool      `json:"completed" gorm:"not null;default:false" bson:"completed"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;autoUpdateTime:false" bson:"updated_at"`
}

// Activity is one entry in a todo's append-only audit log. It has no foreign
// key on purpose: activities outlive the todo they describe.
type Activity struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	TodoID    string    `json:"todo_id" gorm:"type:uuid;not null;index" bson:"todo_id"`
	Action    Action    `json:"action" gorm:"not null" bson:"action"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;autoCreateTime:false" bson:"created_at"`
}

type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Patch is a partial update: a nil field means "leave it alone".
type Patch struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

var (
	ErrNotFound      = errors.New("todo not found")
	ErrInvalidTitle  = errors.New("title must be 1 to 200 characters")
	ErrEmptyPatch    = errors.New("patch must set title or completed")
	ErrInvalidLimit  = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset = errors.New("offset must be 0 or more")
	ErrInvalidJSON   = errors.New("invalid JSON body")
)

const maxTitleLen = 200

// nowUTC is truncated to milliseconds because Mongo stores milliseconds;
// this way both stores round-trip exactly the same value.
func nowUTC() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// newID returns a time-ordered UUIDv7, so sorting by id sorts by creation time.
// NewV7 only fails if crypto/rand fails, and since Go 1.24 that crashes the
// program instead of returning an error, so Must can never panic here.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func validateTitle(raw string) (string, error) {
	title := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

func newTodo(title string) (Todo, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	now := nowUTC()
	return Todo{ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// validatePatch trims the title and rejects a patch that sets nothing.
func validatePatch(p Patch) (Patch, error) {
	if p.Title == nil && p.Completed == nil {
		return Patch{}, ErrEmptyPatch
	}
	if p.Title != nil {
		title, err := validateTitle(*p.Title)
		if err != nil {
			return Patch{}, err
		}
		p.Title = &title
	}
	return p, nil
}

func newActivity(todoID string, action Action, at time.Time) Activity {
	return Activity{ID: newID(), TodoID: todoID, Action: action, CreatedAt: at}
}

// applyPatch changes t in place and returns one activity per real change
// (none if nothing changed). Both stores call it inside their transaction,
// so the "what changed -> which activities" rule is written only once.
func applyPatch(t *Todo, p Patch, now time.Time) []Activity {
	var activities []Activity
	if p.Title != nil && *p.Title != t.Title {
		t.Title = *p.Title
		activities = append(activities, newActivity(t.ID, ActionRenamed, now))
	}
	if p.Completed != nil && *p.Completed != t.Completed {
		t.Completed = *p.Completed
		action := ActionReopened
		if t.Completed {
			action = ActionCompleted
		}
		activities = append(activities, newActivity(t.ID, action, now))
	}
	if len(activities) > 0 {
		t.UpdatedAt = now
	}
	return activities
}
