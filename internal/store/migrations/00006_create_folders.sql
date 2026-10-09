-- +goose Up
CREATE TABLE folders (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE (workspace_id, name_key),
    UNIQUE (id, workspace_id)
);

-- +goose Down
DROP TABLE folders;
