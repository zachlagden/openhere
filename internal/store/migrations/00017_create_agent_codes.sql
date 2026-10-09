-- +goose Up
CREATE TABLE agent_codes (
    email TEXT PRIMARY KEY CHECK (email = lower(email)),
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX agent_codes_expires_at_idx ON agent_codes (expires_at);

-- +goose Down
DROP INDEX agent_codes_expires_at_idx;
DROP TABLE agent_codes;
