package usecase

import "errors"

// These are application rules (about requests), unlike the entity errors (about todos).
var (
	ErrInvalidPagination = errors.New("limit must be 1-100 and offset must not be negative")
	ErrEmptyUpdate       = errors.New("provide title and/or completed")
)
