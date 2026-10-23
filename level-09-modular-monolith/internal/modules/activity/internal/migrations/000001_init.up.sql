-- The activity module's schema. Its data comes only from the todo module's published
-- events, never from the todo tables: there is no foreign key, and no join across schemas.
CREATE SCHEMA activity;

CREATE TABLE activity.activities (
    id         uuid        PRIMARY KEY, -- the event id, so a repeated event is a no-op
    todo_id    uuid        NOT NULL,
    action     text        NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX activities_todo_id_idx ON activity.activities (todo_id);
