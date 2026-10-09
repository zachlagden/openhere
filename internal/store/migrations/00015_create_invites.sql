-- +goose Up
CREATE TABLE invites (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL CHECK (email = lower(email)),
    token_hash TEXT NOT NULL UNIQUE,
    invited_by TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id TEXT REFERENCES workspaces (id) ON DELETE CASCADE,
    role TEXT CHECK (role IN ('admin', 'write', 'read')),
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    revoked_at TIMESTAMP,
    CHECK ((workspace_id IS NULL) = (role IS NULL))
);

CREATE INDEX invites_email_idx ON invites (email);
CREATE INDEX invites_invited_by_idx ON invites (invited_by);

-- +goose Down
DROP INDEX invites_invited_by_idx;
DROP INDEX invites_email_idx;
DROP TABLE invites;
