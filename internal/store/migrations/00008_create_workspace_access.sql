-- +goose Up
CREATE TABLE workspace_access (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'write', 'read')),
    scope_type TEXT NOT NULL CHECK (scope_type IN ('workspace', 'folder', 'site')),
    folder_id TEXT,
    site_id TEXT,
    created_at TIMESTAMP NOT NULL,
    CHECK (
        (scope_type = 'workspace' AND folder_id IS NULL AND site_id IS NULL)
        OR (scope_type = 'folder' AND folder_id IS NOT NULL AND site_id IS NULL)
        OR (scope_type = 'site' AND site_id IS NOT NULL AND folder_id IS NULL)
    ),
    CHECK (scope_type = 'workspace' OR role <> 'admin'),
    FOREIGN KEY (folder_id, workspace_id) REFERENCES folders (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (site_id, workspace_id) REFERENCES sites (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_access_workspace_scope_idx ON workspace_access (workspace_id, user_id) WHERE scope_type = 'workspace';
CREATE UNIQUE INDEX workspace_access_folder_scope_idx ON workspace_access (user_id, folder_id) WHERE scope_type = 'folder';
CREATE UNIQUE INDEX workspace_access_site_scope_idx ON workspace_access (user_id, site_id) WHERE scope_type = 'site';
CREATE INDEX workspace_access_user_id_idx ON workspace_access (user_id);
CREATE INDEX workspace_access_workspace_id_idx ON workspace_access (workspace_id);

-- +goose Down
DROP INDEX workspace_access_workspace_id_idx;
DROP INDEX workspace_access_user_id_idx;
DROP INDEX workspace_access_site_scope_idx;
DROP INDEX workspace_access_folder_scope_idx;
DROP INDEX workspace_access_workspace_scope_idx;
DROP TABLE workspace_access;
