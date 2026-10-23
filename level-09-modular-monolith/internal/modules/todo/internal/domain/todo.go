// Package domain is the todo model in DDD terms: the Todo aggregate, its value objects
// (TodoID, Title), its domain events and its repository interface.
// It is the center of the hexagon: it imports only the standard library and uuid.
package domain

import "time"

// Todo is the aggregate root: the only way to change a todo. Its fields are unexported,
// so every change goes through a method that keeps the rules and records an event.
// It carries no gorm or bson tags: how it is stored is an adapter's concern.
type Todo struct {
	id        TodoID
	title     Title
	completed bool
	deleted   bool
	createdAt time.Time
	updatedAt time.Time
	version   int     // as loaded; 0 = never saved
	events    []Event // recorded since the last save
}

// NewTodo starts a new, not yet completed todo created at now, and records TodoCreated.
// Its version is 0: it has never been saved.
func NewTodo(id TodoID, title Title, now time.Time) *Todo {
	t := &Todo{id: id, title: title, createdAt: now, updatedAt: now}
	t.record(TodoCreated{EventMeta: newEventMeta(id, now), Title: title.String()})
	return t
}

// RehydrateTodo rebuilds a stored todo. Only repositories call it: the state was valid
// when it was saved, so it is trusted and not checked again, and nothing new happened,
// so it records no events. To create a todo, use NewTodo.
func RehydrateTodo(id, title string, completed bool, createdAt, updatedAt time.Time, version int) *Todo {
	return &Todo{
		id:        TodoID{value: id},
		title:     Title{value: title},
		completed: completed,
		createdAt: createdAt,
		updatedAt: updatedAt,
		version:   version,
	}
}

func (t *Todo) ID() TodoID           { return t.id }
func (t *Todo) Title() Title         { return t.title }
func (t *Todo) Completed() bool      { return t.completed }
func (t *Todo) Deleted() bool        { return t.deleted }
func (t *Todo) CreatedAt() time.Time { return t.createdAt }
func (t *Todo) UpdatedAt() time.Time { return t.updatedAt }

// Version is the version the todo had when it was loaded (0 if it was never saved).
// The aggregate never changes it: the repository's optimistic lock compares it with the
// stored version. Save does not refresh it either, so a Todo serves one command; load it
// again for the next one.
func (t *Todo) Version() int { return t.version }

// Rename sets a new title. A title that is already the current one is not a change.
// It takes a Title, not a string, so it cannot fail: the value object is already valid.
func (t *Todo) Rename(title Title, now time.Time) {
	if title == t.title {
		return
	}
	t.title = title
	t.updatedAt = now
	t.record(TodoRenamed{EventMeta: newEventMeta(t.id, now), Title: title.String()})
}

// Complete marks the todo done, unless it already is.
func (t *Todo) Complete(now time.Time) {
	if t.completed {
		return
	}
	t.completed = true
	t.updatedAt = now
	t.record(TodoCompleted{newEventMeta(t.id, now)})
}

// Reopen marks a completed todo as not done, unless it already is.
func (t *Todo) Reopen(now time.Time) {
	if !t.completed {
		return
	}
	t.completed = false
	t.updatedAt = now
	t.record(TodoReopened{newEventMeta(t.id, now)})
}

// Delete marks the todo for removal; the repository deletes it on Save. Its activities stay.
func (t *Todo) Delete(now time.Time) {
	if t.deleted {
		return
	}
	t.deleted = true
	t.record(TodoDeleted{newEventMeta(t.id, now)})
}

// PullEvents returns the events recorded since the last save and clears them.
// The repository calls it in Save, to store the events with the state.
func (t *Todo) PullEvents() []Event {
	events := t.events
	t.events = nil
	return events
}

// record queues an event for the next save.
func (t *Todo) record(e Event) {
	t.events = append(t.events, e)
}
