-- +goose Up
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE CHECK (length(slug) BETWEEN 1 AND 63 AND slug = lower(slug)),
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('personal', 'shared')),
    created_by TEXT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX workspaces_one_personal_per_user ON workspaces (created_by) WHERE kind = 'personal';

-- +goose Down
DROP INDEX workspaces_one_personal_per_user;
DROP TABLE workspaces;
