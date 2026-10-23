package model

import "errors"

// Domain errors. The handler maps them to HTTP status codes with errors.Is,
// so their messages are safe to show to clients.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1 to 200 characters")
	ErrEmptyPatch   = errors.New("patch must set title or completed")
)
