-- +goose Up
INSERT INTO workspaces (id, slug, name, kind, created_by, created_at, updated_at)
SELECT u.id, 'personal-' || substr(u.id, 1, 12), 'Personal', 'personal', u.id, u.created_at, u.created_at
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM workspaces w WHERE w.kind = 'personal' AND w.created_by = u.id);

INSERT INTO workspace_access (id, workspace_id, user_id, role, scope_type, created_at)
SELECT u.id, u.id, u.id, 'admin', 'workspace', u.created_at
FROM users u
WHERE EXISTS (SELECT 1 FROM workspaces w WHERE w.id = u.id AND w.kind = 'personal')
AND NOT EXISTS (SELECT 1 FROM workspace_access a WHERE a.workspace_id = u.id AND a.user_id = u.id AND a.scope_type = 'workspace');

-- +goose Down
DELETE FROM workspace_access WHERE id IN (SELECT id FROM users) AND workspace_id = id;
DELETE FROM workspaces WHERE kind = 'personal' AND id IN (SELECT id FROM users);
