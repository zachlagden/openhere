-- +goose Up
CREATE TABLE site_domains (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    domain TEXT NOT NULL UNIQUE CHECK (domain = lower(domain) AND length(domain) BETWEEN 4 AND 253),
    challenge_token TEXT NOT NULL,
    verified_at TIMESTAMP,
    last_checked_at TIMESTAMP,
    last_check_result TEXT NOT NULL DEFAULT '',
    missing_checks INTEGER NOT NULL DEFAULT 0 CHECK (missing_checks >= 0),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    primary_site_id TEXT UNIQUE REFERENCES sites (id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    claim_started_at TIMESTAMP NOT NULL,
    CHECK (
        (is_primary AND primary_site_id IS NOT NULL AND primary_site_id = site_id AND verified_at IS NOT NULL)
        OR (NOT is_primary AND primary_site_id IS NULL)
    )
);

CREATE INDEX site_domains_site_id_idx ON site_domains (site_id);
CREATE INDEX site_domains_claim_idx ON site_domains (verified_at, claim_started_at);

-- +goose Down
DROP INDEX site_domains_claim_idx;
DROP INDEX site_domains_site_id_idx;
DROP TABLE site_domains;
