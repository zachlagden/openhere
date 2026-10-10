-- +goose Up
DELETE FROM instance_settings WHERE key = 'internal.viewerSecret';

-- +goose Down
SELECT 1;
