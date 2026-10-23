-- The todo module's schema. Everything the module stores lives here, and nothing else does:
-- no other module reads or writes these tables.
CREATE SCHEMA todo;

-- Write side: the Todo aggregate's state.
CREATE TABLE todo.todos (
    id         uuid        PRIMARY KEY,
    title      text        NOT NULL,
    completed  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Optimistic locking: a save only matches the row at the version it loaded.
    version    integer     NOT NULL
);

-- The transactional outbox: every domain event, written in the same transaction as the
-- todo change. The relay publishes each one on the event bus and sets processed_at.
CREATE TABLE todo.outbox (
    id           uuid        PRIMARY KEY, -- the event id (UUIDv7): time-ordered, so also the relay's order
    todo_id      uuid        NOT NULL,
    name         text        NOT NULL,
    payload      jsonb       NOT NULL,
    occurred_at  timestamptz NOT NULL,
    processed_at timestamptz             -- NULL until the relay has handled it
);

-- The relay's poll: unprocessed events in id order. Partial, so it stays as small as the backlog.
CREATE INDEX outbox_unprocessed_idx ON todo.outbox (id) WHERE processed_at IS NULL;

-- Read side: the read model, written only by the projector. No foreign key: it is derived data.
CREATE TABLE todo.todo_views (
    id         uuid        PRIMARY KEY,
    title      text        NOT NULL,
    completed  boolean     NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
