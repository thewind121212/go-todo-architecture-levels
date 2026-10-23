// Package model holds the plain data types and the business rules that need no
// storage: title validation and which activities a change produces.
package model

import (
	"fmt"
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

// Patch is a partial update. A nil field means "leave unchanged".
type Patch struct {
	Title     *string
	Completed *bool
}

// NewTodo validates title and returns a new, not yet completed todo.
func NewTodo(title string, now time.Time) (Todo, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return Todo{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Todo{}, fmt.Errorf("new todo id: %w", err)
	}
	return Todo{ID: id.String(), Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// Apply changes t according to p and returns one action per field that really
// changed, in a fixed order (renamed before completed/reopened). No actions
// means nothing changed, and t is left untouched.
//
// This rule lives in model rather than in the service because it is pure: it
// needs no repository or transaction, so it can be tested on its own and the
// service stays focused on orchestration.
func (t *Todo) Apply(p Patch, now time.Time) ([]Action, error) {
	if p.Title == nil && p.Completed == nil {
		return nil, ErrEmptyPatch
	}
	var actions []Action
	if p.Title != nil {
		title, err := normalizeTitle(*p.Title)
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
		t.UpdatedAt = now
	}
	return actions, nil
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleLen {
		return "", ErrInvalidTitle
	}
	return title, nil
}
