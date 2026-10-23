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

const maxTitleLen = 200

// Domain errors. The handler maps them to HTTP status codes with errors.Is,
// so their messages are safe to show to clients.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1 to 200 characters")
	ErrEmptyPatch   = errors.New("patch must set title or completed")
)

// Action says what happened to a todo. The names belong to this feature; the
// activity feature stores them as plain strings.
type Action string

const (
	ActionCreated   Action = "created"
	ActionRenamed   Action = "renamed"
	ActionCompleted Action = "completed"
	ActionReopened  Action = "reopened"
	ActionDeleted   Action = "deleted"
)

// Todo has no storage or JSON tags: the adapters and the handler map it to
// their own shapes.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Patch is a partial update. A nil field means "leave unchanged".
type Patch struct {
	Title     *string
	Completed *bool
}

// New validates title and returns a new, not yet completed todo.
func New(title string, now time.Time) (Todo, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Todo{}, fmt.Errorf("new todo id: %w", err)
	}
	return Todo{ID: id.String(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// ValidatePatch checks p before any database call and returns it with the
// title trimmed, so Apply only has to compare.
func ValidatePatch(p Patch) (Patch, error) {
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

// Apply changes t according to a validated patch and returns one action per
// field that really changed, renamed first. No actions means nothing changed.
func (t *Todo) Apply(p Patch, now time.Time) []Action {
	var actions []Action
	if p.Title != nil && *p.Title != t.Title {
		t.Title = *p.Title
		actions = append(actions, ActionRenamed)
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
		t.UpdatedAt = now
	}
	return actions
}

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}
