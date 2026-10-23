package entity

import "errors"

// Sentinel errors are part of the business core, so every outer layer can check them with errors.Is
// without knowing which database or framework produced them.
var (
	ErrNotFound     = errors.New("todo not found")
	ErrInvalidTitle = errors.New("title must be 1-200 characters")
)
