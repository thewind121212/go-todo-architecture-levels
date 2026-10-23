package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"example.com/todo/level09/internal/modules/todo/contracts"
	"example.com/todo/level09/internal/platform/eventbus"
)

// Service records todo events as activities and lists them.
type Service struct {
	activities ActivityLog
}

var _ ActivityQueries = (*Service)(nil)

func NewService(activities ActivityLog) *Service {
	return &Service{activities: activities}
}

// Subscribe registers Record for every todo event.
func (s *Service) Subscribe(bus *eventbus.Bus) {
	for _, name := range []string{
		contracts.TodoCreatedName, contracts.TodoRenamedName, contracts.TodoCompletedName,
		contracts.TodoReopenedName, contracts.TodoDeletedName,
	} {
		bus.Subscribe(name, s.Record)
	}
}

// Record is the module's subscriber: one todo event becomes one activity. It commits in the
// activity module's own storage, never in the todo module's or the relay's transaction. It is
// one insert, so it is atomic by itself; if handling an event ever needs several writes, wrap
// them in this module's own transaction. Add ignores an id it already has, so a redelivered
// event changes nothing.
func (s *Service) Record(ctx context.Context, event eventbus.Event) error {
	activity, err := toActivity(event)
	if err != nil {
		return err
	}
	return s.activities.Add(ctx, activity)
}

// ListActivities returns a todo's audit log, oldest first. It does not check that the todo
// exists: that would mean asking the todo module, and the log outlives deleted todos anyway.
func (s *Service) ListActivities(ctx context.Context, rawTodoID string) ([]*Activity, error) {
	id, err := uuid.Parse(rawTodoID)
	if err != nil {
		return nil, ErrNotFound // a malformed id names no todo
	}
	return s.activities.ListByTodo(ctx, id.String())
}

// toActivity reads a todo event in this module's own words. It is the activity module's
// side of the contract: it depends on the published events only, not on how the todo
// module models a todo.
func toActivity(event eventbus.Event) (*Activity, error) {
	var meta contracts.EventMeta
	var action string
	switch e := event.(type) {
	case contracts.TodoCreated:
		meta, action = e.EventMeta, "created"
	case contracts.TodoRenamed:
		meta, action = e.EventMeta, "renamed"
	case contracts.TodoCompleted:
		meta, action = e.EventMeta, "completed"
	case contracts.TodoReopened:
		meta, action = e.EventMeta, "reopened"
	case contracts.TodoDeleted:
		meta, action = e.EventMeta, "deleted"
	default:
		return nil, fmt.Errorf("record activity: unknown event %T", event)
	}
	return &Activity{ID: meta.EventID, TodoID: meta.TodoID, Action: action, CreatedAt: meta.OccurredAt}, nil
}
