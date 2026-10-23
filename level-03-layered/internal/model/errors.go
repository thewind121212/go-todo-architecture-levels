package model

import "errors"

// Sentinel errors shared by every layer. The handler maps them to HTTP status
// codes with errors.Is, so their messages are safe to show to clients.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1-200 characters")
	ErrEmptyPatch   = errors.New("provide title and/or completed")
)
