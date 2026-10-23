package domain

import "errors"

var (
	// ErrNotFound means no todo has the given id (malformed ids included).
	ErrNotFound = errors.New("todo not found")

	// ErrInvalidTitle means the trimmed title is empty or longer than 200 characters.
	ErrInvalidTitle = errors.New("title must be 1-200 characters")

	// ErrEmptyUpdate means an update asked to change nothing at all.
	ErrEmptyUpdate = errors.New("provide title and/or completed")
)
