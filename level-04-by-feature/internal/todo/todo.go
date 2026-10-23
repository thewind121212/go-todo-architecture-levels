// Package todo is the todo feature: entity, rules, service, storage and HTTP,
// all in one folder.
package todo

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxTitleLen = 200
	maxLimit    = 100
)

var (
	ErrNotFound      = errors.New("todo not found")
	ErrInvalidTitle  = errors.New("title must be 1-200 characters")
	ErrEmptyUpdate   = errors.New("nothing to update: send title and/or completed")
	ErrInvalidLimit  = errors.New("limit must be between 1 and 100")
	ErrInvalidOffset = errors.New("offset must be 0 or greater")
)

// Activity actions. They describe what happened to a todo, so this feature
// owns them; the activity feature stores them as plain strings.
const (
	ActionCreated   = "created"
	ActionRenamed   = "renamed"
	ActionCompleted = "completed"
	ActionReopened  = "reopened"
	ActionDeleted   = "deleted"
)

// Todo carries its JSON, GORM and BSON tags itself: one struct is the
// simplest thing that works until a later level separates them.
type Todo struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey" bson:"_id"`
	Title     string    `json:"title" gorm:"not null" bson:"title"`
	Completed bool      `json:"completed" gorm:"not null;default:false" bson:"completed"`
	CreatedAt time.Time `json:"created_at" gorm:"not null" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null" bson:"updated_at"`
}

// Patch is a partial update; nil fields stay unchanged.
type Patch struct {
	Title     *string
	Completed *bool
}

// New builds a valid, not yet completed todo.
func New(title string) (Todo, error) {
	title, err := cleanTitle(title)
	if err != nil {
		return Todo{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Todo{}, fmt.Errorf("new todo id: %w", err)
	}
	now := now()
	return Todo{ID: id.String(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// Apply changes t and returns one action per real change, renamed first.
// No actions means nothing changed.
func (t *Todo) Apply(p Patch) ([]string, error) {
	var actions []string
	if p.Title != nil {
		title, err := cleanTitle(*p.Title)
		if err != nil {
			return nil, err
		}
		if title != t.Title {
			t.Title = title
			actions = append(actions, ActionRenamed)
		}
	}
	if p.Completed != nil && *p.Completed != t.Completed {
		t.Completed = *p.Completed
		if t.Completed {
			actions = append(actions, ActionCompleted)
		} else {
			actions = append(actions, ActionReopened)
		}
	}
	if len(actions) > 0 {
		t.UpdatedAt = now()
	}
	return actions, nil
}

func cleanTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

// parseID returns the canonical form of id. A malformed id can never match a
// todo, so it is reported as not found.
func parseID(id string) (string, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return "", ErrNotFound
	}
	return u.String(), nil
}

// now is truncated to milliseconds because Mongo stores milliseconds; both
// adapters then round-trip identical values.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
