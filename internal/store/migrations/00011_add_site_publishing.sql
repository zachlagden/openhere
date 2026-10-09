-- +goose Up
ALTER TABLE sites ADD COLUMN current_version_id TEXT REFERENCES versions (id);
ALTER TABLE sites ADD COLUMN spa_mode BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX sites_current_version_id_idx ON sites (current_version_id);

-- +goose Down
DROP INDEX sites_current_version_id_idx;
ALTER TABLE sites DROP COLUMN spa_mode;
ALTER TABLE sites DROP COLUMN current_version_id;
