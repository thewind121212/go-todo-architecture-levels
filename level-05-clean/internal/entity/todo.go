// Package entity holds the enterprise business rules: the Todo and Activity types and the rules that
// apply to them. It is the innermost circle, so it imports only the standard library and uuid.
package entity

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxTitleLength = 200

// Todo carries no gorm or bson tags: how it is stored is an adapter's concern, not the entity's.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTodo validates the title and returns a new, not yet completed todo.
func NewTodo(title string) (Todo, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return Todo{}, err
	}
	now := now()
	return Todo{ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename sets a new title and reports whether the title actually changed.
func (t *Todo) Rename(title string) (bool, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return false, err
	}
	if title == t.Title {
		return false, nil
	}
	t.Title = title
	t.UpdatedAt = now()
	return true, nil
}

// SetCompleted sets the completed flag. It returns the action that describes the change,
// or false if the flag already had that value.
func (t *Todo) SetCompleted(completed bool) (Action, bool) {
	if completed == t.Completed {
		return "", false
	}
	t.Completed = completed
	t.UpdatedAt = now()
	if completed {
		return ActionCompleted, true
	}
	return ActionReopened, true
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLength {
		return "", ErrInvalidTitle
	}
	return title, nil
}

// now is truncated to milliseconds because Mongo stores milliseconds; this way both adapters
// round-trip exactly the same value.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// newID returns a time-ordered UUIDv7, so sorting by ID sorts by creation. NewV7 only fails if the
// OS random source fails, which the Go runtime already treats as fatal.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}
