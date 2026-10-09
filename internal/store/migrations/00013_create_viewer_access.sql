-- +goose Up
CREATE TABLE viewer_codes (
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    PRIMARY KEY (site_id, email)
);

CREATE INDEX viewer_codes_expires_at_idx ON viewer_codes (expires_at);

CREATE TABLE viewer_handoffs (
    token_hash TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX viewer_handoffs_expires_at_idx ON viewer_handoffs (expires_at);

-- +goose Down
DROP INDEX viewer_handoffs_expires_at_idx;
DROP TABLE viewer_handoffs;
DROP INDEX viewer_codes_expires_at_idx;
DROP TABLE viewer_codes;
