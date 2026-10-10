-- +goose Up
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN disabled_at TIMESTAMP;

-- +goose Down
ALTER TABLE users DROP COLUMN disabled_at;
ALTER TABLE users DROP COLUMN display_name;
