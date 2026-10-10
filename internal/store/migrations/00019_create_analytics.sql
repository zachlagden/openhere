-- +goose Up
ALTER TABLE sites ADD COLUMN analytics_enabled BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE analytics_events (
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    occurred_at TIMESTAMP NOT NULL,
    day TEXT NOT NULL,
    hour SMALLINT NOT NULL CHECK (hour BETWEEN 0 AND 23),
    path TEXT NOT NULL,
    status_class TEXT NOT NULL CHECK (status_class IN ('2xx', '4xx')),
    referrer_host TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    browser TEXT NOT NULL,
    os TEXT NOT NULL,
    device TEXT NOT NULL CHECK (device IN ('desktop', 'mobile', 'tablet', 'bot', 'other')),
    visitor TEXT
);

CREATE INDEX analytics_events_site_day_idx ON analytics_events (site_id, day);
CREATE INDEX analytics_events_day_idx ON analytics_events (day);

CREATE TABLE analytics_daily (
    site_id TEXT NOT NULL REFERENCES sites (id) ON DELETE CASCADE,
    day TEXT NOT NULL,
    dimension TEXT NOT NULL,
    value TEXT NOT NULL,
    views BIGINT NOT NULL CHECK (views >= 0),
    uniques BIGINT NOT NULL CHECK (uniques >= 0),
    PRIMARY KEY (site_id, day, dimension, value)
);

CREATE INDEX analytics_daily_day_idx ON analytics_daily (day);

-- +goose Down
DROP INDEX analytics_daily_day_idx;
DROP TABLE analytics_daily;
DROP INDEX analytics_events_day_idx;
DROP INDEX analytics_events_site_day_idx;
DROP TABLE analytics_events;
ALTER TABLE sites DROP COLUMN analytics_enabled;
