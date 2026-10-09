-- +goose Up
CREATE TABLE sites (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE RESTRICT,
    folder_id TEXT REFERENCES folders (id) ON DELETE SET NULL,
    slug TEXT NOT NULL UNIQUE CHECK (length(slug) BETWEEN 1 AND 63 AND slug = lower(slug)),
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by TEXT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE (id, workspace_id)
);

CREATE INDEX sites_workspace_id_idx ON sites (workspace_id);
CREATE INDEX sites_folder_id_idx ON sites (folder_id);
CREATE INDEX sites_created_at_idx ON sites (created_at, id);

-- +goose Down
DROP INDEX sites_created_at_idx;
DROP INDEX sites_folder_id_idx;
DROP INDEX sites_workspace_id_idx;
DROP TABLE sites;
