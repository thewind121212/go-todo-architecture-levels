-- Write side: the Todo aggregate's state.
CREATE TABLE todos (
    id         uuid        PRIMARY KEY,
    title      text        NOT NULL,
    completed  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Optimistic locking: a save only matches the row at the version it loaded.
    version    integer     NOT NULL
);

-- The transactional outbox: every domain event, written in the same transaction as the
-- todo change. The relay projects each one into the read models and sets processed_at.
CREATE TABLE outbox (
    id           uuid        PRIMARY KEY, -- the event id (UUIDv7): time-ordered, so also the relay's order
    todo_id      uuid        NOT NULL,
    name         text        NOT NULL,
    payload      jsonb       NOT NULL,
    occurred_at  timestamptz NOT NULL,
    processed_at timestamptz             -- NULL until the relay has handled it
);

-- The relay's poll: unprocessed events in id order. Partial, so it stays as small as the backlog.
CREATE INDEX outbox_unprocessed_idx ON outbox (id) WHERE processed_at IS NULL;

-- Read side: the read models, written only by the projector. No foreign keys: they are
-- derived data, and activities outlive deleted todos.
CREATE TABLE todo_views (
    id         uuid        PRIMARY KEY,
    title      text        NOT NULL,
    completed  boolean     NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE activity_views (
    id         uuid        PRIMARY KEY, -- the event id, so a repeated event is a no-op
    todo_id    uuid        NOT NULL,
    action     text        NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX activity_views_todo_id_idx ON activity_views (todo_id);
