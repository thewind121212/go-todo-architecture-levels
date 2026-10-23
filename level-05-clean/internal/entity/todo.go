// Package entity holds the enterprise business rules: the Todo and Activity types and the rules that
// apply to them. It is the innermost circle, so it imports only the standard library and uuid.
package entity

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxTitleLen = 200

// Todo carries no tags: storage and JSON shapes belong to the adapters.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewTodo validates title and returns a new, not yet completed todo created at now.
func NewTodo(title string, now time.Time) (Todo, error) {
	title, err := ValidateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Todo{}, fmt.Errorf("new todo id: %w", err)
	}
	return Todo{ID: id.String(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
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

// ValidateTitle returns the trimmed title, or ErrInvalidTitle. It is exported so a use case can
// reject a bad title before it opens a transaction.
func ValidateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}
