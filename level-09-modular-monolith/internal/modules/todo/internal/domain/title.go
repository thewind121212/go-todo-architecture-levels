package domain

import (
	"strings"
	"unicode/utf8"
)

const maxTitleLen = 200

// Title is what a Todo says. It is a value object: NewTitle is the only way to
// make one, so a Title you hold is always trimmed and 1 to 200 characters long.
type Title struct {
	value string
}

// NewTitle trims raw and checks its length in runes, or returns ErrInvalidTitle.
func NewTitle(raw string) (Title, error) {
	s := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(s); n < 1 || n > maxTitleLen {
		return Title{}, ErrInvalidTitle
	}
	return Title{value: s}, nil
}

func (t Title) String() string {
	return t.value
}
