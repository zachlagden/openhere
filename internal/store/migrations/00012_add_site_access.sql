-- +goose Up
ALTER TABLE sites ADD COLUMN access_mode TEXT NOT NULL DEFAULT 'public' CHECK (access_mode IN ('public', 'password', 'restricted'));
ALTER TABLE sites ADD COLUMN access_password_hash TEXT;
ALTER TABLE sites ADD COLUMN access_epoch BIGINT NOT NULL DEFAULT 0 CHECK (access_epoch >= 0);

CREATE TABLE site_access_emails (
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    email TEXT NOT NULL CHECK (email = lower(email)),
    PRIMARY KEY (site_id, email)
);

CREATE TABLE site_access_domains (
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    domain TEXT NOT NULL CHECK (domain = lower(domain)),
    PRIMARY KEY (site_id, domain)
);

-- +goose Down
DROP TABLE site_access_domains;
DROP TABLE site_access_emails;
ALTER TABLE sites DROP COLUMN access_epoch;
ALTER TABLE sites DROP COLUMN access_password_hash;
ALTER TABLE sites DROP COLUMN access_mode;
