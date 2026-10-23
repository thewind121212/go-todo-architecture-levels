package domain

import "errors"

// Sentinel errors are part of the core, so every adapter can check them with errors.Is
// without knowing which database or framework produced them.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1 to 200 characters")
	ErrEmptyPatch   = errors.New("patch must set title or completed")
)
