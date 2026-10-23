// Package domain holds the todo feature's entities and rules.
// It is the center of the hexagon: it imports nothing from the outside world.
package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxTitleLen = 200

// Todo is the core entity of this feature.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTodo validates the title and returns a new, not completed todo.
func NewTodo(title string) (*Todo, error) {
	title, err := cleanTitle(title)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := now()
	return &Todo{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// Rename sets a new title. It reports whether the title actually changed.
func (t *Todo) Rename(title string) (bool, error) {
	title, err := cleanTitle(title)
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

// SetCompleted sets the completed flag. It reports whether the flag actually changed.
func (t *Todo) SetCompleted(completed bool) bool {
	if completed == t.Completed {
		return false
	}
	t.Completed = completed
	t.UpdatedAt = now()
	return true
}

// IsValidID reports whether id is a UUID in the canonical form this app generates.
// Being strict keeps both adapters identical: Postgres would also accept other
// spellings ("{...}", upper case), Mongo compares the string as is.
func IsValidID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == id
}

func cleanTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}

// newID returns a UUIDv7: time-ordered, so sorting by id sorts by creation time.
func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return id.String(), nil
}

// now is truncated to milliseconds because Mongo stores milliseconds;
// both adapters then round-trip identical values.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
