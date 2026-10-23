package domain

import "github.com/google/uuid"

// TodoID identifies a Todo. It is a value object: immutable, and two TodoIDs
// with the same value are equal (==). Holding one means the id is well-formed.
type TodoID struct {
	value string
}

// NewTodoID returns a fresh, time-ordered id, so sorting by id sorts by creation.
func NewTodoID() TodoID {
	return TodoID{value: newID()}
}

// ParseTodoID accepts any UUID and keeps its canonical form, or returns ErrInvalidID.
func ParseTodoID(s string) (TodoID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return TodoID{}, ErrInvalidID
	}
	return TodoID{value: id.String()}, nil
}

func (id TodoID) String() string {
	return id.value
}

// newID returns a UUIDv7 string. NewV7 only fails if the OS random source fails,
// which the Go runtime already treats as fatal.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}
