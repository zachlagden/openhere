-- +goose Up
CREATE TABLE versions (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'finalized')),
    created_by TEXT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL,
    finalized_at TIMESTAMP,
    file_count INTEGER NOT NULL CHECK (file_count >= 0),
    total_bytes BIGINT NOT NULL CHECK (total_bytes >= 0),
    base_version_id TEXT,
    CHECK ((status = 'finalized') = (finalized_at IS NOT NULL)),
    UNIQUE (id, site_id)
);

CREATE INDEX versions_site_created_idx ON versions (site_id, created_at, id);
CREATE INDEX versions_status_created_idx ON versions (status, created_at);

CREATE TABLE version_files (
    version_id TEXT NOT NULL REFERENCES versions (id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    content_type TEXT NOT NULL,
    PRIMARY KEY (version_id, path)
);

-- +goose Down
DROP TABLE version_files;
DROP INDEX versions_status_created_idx;
DROP INDEX versions_site_created_idx;
DROP TABLE versions;
