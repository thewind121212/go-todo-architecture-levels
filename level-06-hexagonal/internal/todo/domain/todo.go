// Package domain holds the todo feature's entities and rules.
// It is the center of the hexagon: it imports only the standard library and uuid.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxTitleLen = 200

// Todo carries no gorm or bson tags: how it is stored is an adapter's concern.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTodo validates the title and returns a new, not yet completed todo created at now.
func NewTodo(title string, now time.Time) (*Todo, error) {
	title, err := ValidateTitle(title)
	if err != nil {
		return nil, err
	}
	return &Todo{ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename sets a new title and reports whether the title actually changed.
// It validates again so a Todo can never hold an invalid title, whoever calls it.
func (t *Todo) Rename(title string, now time.Time) (bool, error) {
	title, err := ValidateTitle(title)
	if err != nil {
		return false, err
	}
	if title == t.Title {
		return false, nil
	}
	t.Title = title
	t.UpdatedAt = now
	return true, nil
}

// SetCompleted sets the completed flag. It returns the action that describes the change,
// or false if the flag already had that value.
func (t *Todo) SetCompleted(completed bool, now time.Time) (Action, bool) {
	if completed == t.Completed {
		return "", false
	}
	t.Completed = completed
	t.UpdatedAt = now
	if completed {
		return ActionCompleted, true
	}
	return ActionReopened, true
}

// ValidateTitle returns the trimmed title, or ErrInvalidTitle. It is exported so the service can
// reject a bad title before it opens a transaction.
func ValidateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

// newID returns a time-ordered UUIDv7, so sorting by ID sorts by creation. NewV7 only fails if the
// OS random source fails, which the Go runtime already treats as fatal.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}
