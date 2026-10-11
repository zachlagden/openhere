# Upgrading and backups

## What is on disk

With SQLite everything is in `DATA_DIR` (`/data` in the Docker image):

| Path | Contents |
|---|---|
| `openhere.db`, `openhere.db-wal`, `openhere.db-shm` | The SQLite database in WAL mode. Absent with PostgreSQL. |
| `sites/<siteId>/versions/<versionId>/files/` | The files of each version. A finalized version never changes. |
| `tmp/` | Upload files in progress. The hourly maintenance task removes stale ones after 24 hours. |
| `secret.key` | The key that seals stored secrets, unless `SECRET_KEY` is set. Mode 0600. |
| `geo/GeoLite2-City.mmdb` | The MaxMind database, if you configured lookups. It is downloaded again when missing. |
| `certs/` | ACME account key and certificates with built-in HTTPS, unless `TLS_STORAGE` points elsewhere. |

With PostgreSQL the database holds everything the SQLite file would, and `DATA_DIR` still holds `sites/`, `tmp/`, `secret.key`, `geo/` and `certs/`.

The database also stores two internal secrets in the `instance_settings` table: the viewer cookie signing key and the analytics visitor key. A restore from backup brings them back, so viewers stay signed in and visitor hashes stay stable.

## Backups

A backup has to capture the database and `DATA_DIR` as they were at the same moment. The database says which versions exist, and the hourly maintenance task deletes any site or version directory that has no database row. A backup taken from two different times can therefore lose files. Restore the database and the data directory from the same backup, and never restore only one of them onto a running instance.

### SQLite

The simplest correct backup stops the server so the database file is quiescent:

```sh
docker stop -t 30 openhere
docker run --rm -v openhere-data:/data:ro -v "$PWD":/backup alpine \
  tar -czf /backup/openhere-$(date +%F).tar.gz -C /data .
docker start openhere
```

For a systemd install, run `systemctl stop openhere`, `tar -czf openhere-$(date +%F).tar.gz -C /var/lib/openhere .` and `systemctl start openhere`.

The archive includes `secret.key`. Whoever reads it can decrypt the stored secrets (see [Security](security.md#secrets-at-rest)). Encrypt the archive, or set `SECRET_KEY` from outside the data directory and store that value somewhere else.

To back up while the server runs, copy the database with SQLite's own backup command (`sqlite3 openhere.db ".backup openhere-copy.db"`), never with `cp`, because the WAL file holds committed data. Then copy `sites/` afterwards. A version that a publish adds between the two steps is missing from the copied database and is then deleted as an orphan if you restore it, which loses only that publish. A version deleted in between is missing from the copied files, which is a problem only if you restore and want that version back.

### PostgreSQL

```sh
docker exec openhere-db pg_dump -U openhere -Fc openhere > openhere-$(date +%F).dump
docker run --rm -v openhere-data:/data:ro -v "$PWD":/backup alpine \
  tar -czf /backup/openhere-data-$(date +%F).tar.gz -C /data .
```

Dump the database first and the data directory second. Site files in a finalized version never change, so a data copy taken after the dump contains every version the dump refers to, unless a version was deleted in between.

### Keeping the sealing key

Keep a copy of `secret.key` or of your `SECRET_KEY` value apart from the data. Without it, the stored SMTP password, Resend key, Cloudflare token and MaxMind key cannot be read, email stays off, and you re-enter them as root. Agent sign-in codes in flight stop working.

## Restoring

1. Stop the server. Start from an empty data directory (and an empty database for PostgreSQL).
2. Restore the database and the data directory from the same backup.
3. Start the server. It applies any migrations that the backup predates.

```sh
docker volume create openhere-data-restored
docker run --rm -v openhere-data-restored:/data -v "$PWD":/backup alpine \
  tar -xzf /backup/openhere-2026-01-01.tar.gz -C /data
docker run --rm -v openhere-data-restored:/data alpine chown -R 65532:65532 /data
```

For PostgreSQL, restore the dump into the database before you start the server:

```sh
docker exec -i openhere-db pg_restore -U openhere -d openhere --clean --if-exists < openhere-2026-01-01.dump
```

## Upgrading

1. Read the changelog for the release.
2. Take a backup as above.
3. Replace the binary, or build or pull the new image, and restart with the same data.

The server applies its database migrations on start, before it serves anything. The migrations are embedded in the binary and run in order. A start that cannot migrate fails with `migrate store: ...` and serves nothing.

There is no downgrade command. Starting an older binary against a database that a newer binary migrated is unsupported. To go back, restore the backup you took before the upgrade.

Do not run two server versions against one `DATA_DIR` at once, for example during a rolling deploy. The upload and finalize locks live in process memory, and two processes on one directory can race. Stop the old container before you start the new one.
