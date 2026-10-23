// Package model holds the plain data types and the business rules that need no
// storage: title validation and which activities a change produces.
package model

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxTitleLen = 200

// Todo is the data the app stores and returns. It has no storage or JSON tags:
// each adapter and the handler map it to their own shapes.
type Todo struct {
	ID        string
	Title     string
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Patch is a partial update: a nil field means "leave it alone".
type Patch struct {
	Title     *string
	Completed *bool
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

// NewTodo validates title and returns a new, not yet completed todo.
func NewTodo(title string, now time.Time) (Todo, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Todo{}, err
	}
	return Todo{ID: newID(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// ValidatePatch trims the title and rejects a patch that sets nothing. The
// service calls it before opening a transaction, so bad input never reaches
// the database.
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
// real change (none if nothing changed), renamed before completed/reopened.
//
// This rule lives in model rather than in the service because it is pure: it
// needs no repository or transaction, so it can be tested on its own and the
// service stays focused on orchestration.
func (t *Todo) Apply(p Patch, now time.Time) []Action {
	var actions []Action
	if p.Title != nil && *p.Title != t.Title {
		t.Title = *p.Title
		actions = append(actions, ActionRenamed)
	}
	if p.Completed != nil && *p.Completed != t.Completed {
		t.Completed = *p.Completed
		action := ActionReopened
		if t.Completed {
			action = ActionCompleted
		}
		actions = append(actions, action)
	}
	if len(actions) > 0 {
		t.UpdatedAt = now
	}
	return actions
}
