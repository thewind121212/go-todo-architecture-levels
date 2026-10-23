CREATE TABLE todos (
    id         uuid        PRIMARY KEY,
    title      text        NOT NULL,
    completed  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    -- Optimistic locking: a save only matches the row at the version it loaded.
    version    integer     NOT NULL
);

-- The persisted domain events. No foreign key to todos: activities outlive deleted todos.
CREATE TABLE activities (
    id         uuid        PRIMARY KEY,
    todo_id    uuid        NOT NULL,
    action     text        NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX activities_todo_id_idx ON activities (todo_id);
