package domain

import "context"

// TodoRepository stores Todo aggregates. In DDD the repository is part of the model
// (one per aggregate), so its interface lives here, next to the aggregate it stores;
// the Postgres and Mongo adapters implement it. It is the write side only: queries read
// the read models instead, so there is no List here.
type TodoRepository interface {
	// Get returns ErrNotFound for an unknown id.
	Get(ctx context.Context, id TodoID) (*Todo, error)
	// Save writes the todo's state (inserted, updated or deleted) and appends its pulled
	// events to the outbox, in one transaction. If the stored todo is no longer at the
	// version this one was loaded at, Save returns ErrConflict and writes nothing.
	// A todo with no pending events is not written at all. Save stores version
	// Version()+1 but does not update the todo in memory: load it again for the next change.
	Save(ctx context.Context, todo *Todo) error
}
