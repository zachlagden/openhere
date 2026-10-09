-- +goose Up
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP
);

CREATE INDEX api_keys_user_id_idx ON api_keys (user_id);

-- +goose Down
DROP INDEX api_keys_user_id_idx;
DROP TABLE api_keys;
