# Install with Docker

The project ships a `Dockerfile` and nothing else for container deployment. It deliberately has no Docker Compose file: a compose file would invite a second, drifting description of how to run the server. Every example here is a plain `docker run`, and a platform such as Coolify builds from the `Dockerfile` directly.

## The image

The `Dockerfile` builds the dashboard with Node, builds the Go binary with `CGO_ENABLED=0`, and copies the binary into a distroless static image that runs as the non-root user `nonroot` (uid 65532). It builds for the platform you ask for without emulation.

| Item | Value |
|---|---|
| Entry point | `/openhere` |
| Data directory | `/data`, declared as a `VOLUME`, owned by `nonroot` |
| Default listener | `:8080` (`LISTEN_ADDR`) |
| Ports declared | 8080, and 80 and 443 for built-in HTTPS |

Build the image from a clone of the repository:

```sh
docker build -t openhere .
```

Pass `--build-arg VERSION=1.2.3` to set the version that `GET /api/v1/instance` reports. It defaults to `dev`. For another architecture, use buildx: `docker buildx build --platform linux/arm64 -t openhere .`.

Tagged releases publish a multi-architecture image (linux/amd64 and linux/arm64) to GitHub Container Registry under `ghcr.io/zachlagden/openhere`, tagged with the exact version, the minor version and `latest`. No release exists yet, so build from source until one does.

## Quick start with SQLite

```sh
docker run -d --name openhere \
  -p 8080:8080 \
  -v openhere-data:/data \
  openhere
```

SQLite is the default. The database file, the published sites and the generated `secret.key` all live in the `openhere-data` volume.

A new instance has no users. While the database has none, the server generates a setup token at startup and logs it once:

```sh
docker logs openhere
```

```json
{"level":"INFO","msg":"setup required: send this token with the first setup request","setup_path":"/api/v1/setup","setup_token":"..."}
```

Open `http://localhost:8080`, enter the token, an email address and a password of 12 to 128 characters. The first user becomes the root user. The server discards the token after setup, and a later restart logs no token. You can also create the root user from a shell:

```sh
curl -X POST http://localhost:8080/api/v1/setup \
  -H 'Content-Type: application/json' \
  -d '{"email":"alex@example.com","password":"a password of 12 or more characters","setupToken":"<token from the log>"}'
```

**Reach the server by the name in `PUBLIC_URL`.** Without `PUBLIC_URL` the server assumes `http://localhost:8080`. A request with another `Host`, such as `http://127.0.0.1:8080` or the machine's IP address, is treated as a request to an unknown site host. It returns a 404, or a 405 for a `POST`, and the dashboard does not load. On a server you reach by IP address or another name, set `PUBLIC_URL` to the address you use, and make it match the port you publish:

```sh
docker run -d --name openhere \
  -p 8080:8080 \
  -v openhere-data:/data \
  -e PUBLIC_URL=http://203.0.113.10:8080 \
  openhere
```

For anything beyond local use, put the server behind HTTPS (see below).

## Volumes and permissions

Use a named volume, as above. A named volume takes the ownership of `/data` in the image, so the non-root user can write to it.

If you bind-mount a host directory, make it writable by uid 65532 first. A root-owned directory makes the server stop with `open store: ping sqlite: unable to open database file`:

```sh
mkdir -p /srv/openhere/data
sudo chown 65532:65532 /srv/openhere/data
docker run -d --name openhere -p 8080:8080 -v /srv/openhere/data:/data openhere
```

Keep `/data` on persistent storage. [Upgrading and backups](upgrading-and-backups.md) describes what is in it.

## PostgreSQL

Set `DATABASE_URL` to use PostgreSQL. The server applies its migrations on start. `DATA_DIR` still holds the published site files and `secret.key`, so keep the volume.

```sh
docker network create openhere-net

docker run -d --name openhere-db --network openhere-net \
  -e POSTGRES_USER=openhere \
  -e POSTGRES_PASSWORD=change-me \
  -e POSTGRES_DB=openhere \
  -v openhere-pg:/var/lib/postgresql/data \
  postgres:17-alpine

docker run -d --name openhere --network openhere-net \
  -p 8080:8080 \
  -v openhere-data:/data \
  -e DATABASE_URL=postgres://openhere:change-me@openhere-db:5432/openhere \
  openhere
```

Replace `change-me` with a real password. The URL must start with `postgres://` or `postgresql://` and name a host. Use `?sslmode=require` on the URL when the database is on another machine. Back up the database and the data volume together (see [Upgrading and backups](upgrading-and-backups.md)).

## HTTPS

Two options:

1. **A reverse proxy** such as Traefik (Coolify runs one) or Caddy terminates TLS and forwards to port 8080. See [Reverse proxy](reverse-proxy.md).
2. **Built-in HTTPS.** The server obtains its own certificates from an ACME certificate authority. Publish ports 80 and 443 and set the TLS variables:

```sh
docker run -d --name openhere \
  -p 80:80 -p 443:443 \
  -v openhere-data:/data \
  -e TLS_ENABLED=true \
  -e PUBLIC_URL=https://sites.example.com \
  -e ACME_EMAIL=alex@example.com \
  -e ACME_CA=https://acme-staging-v02.api.letsencrypt.org/directory \
  openhere
```

With `TLS_ENABLED=true` the server ignores `LISTEN_ADDR` and listens on `:80` (challenges and redirect) and `:443` (HTTPS). Ports 80 and 443 bind inside Docker without extra setup. Test with the staging CA first, then remove `ACME_CA`. The DNS name in `PUBLIC_URL` must already point at the machine. See [Built-in HTTPS](built-in-https.md).

## Environment and secrets

Pass configuration with `-e`, or `--env-file` with a file outside version control. Supply `SECRET_KEY` from outside the data volume if backups of that volume must not be able to decrypt stored secrets. See [Security](security.md#secrets-at-rest) and [Configuration](configuration.md).

```sh
docker run -d --name openhere \
  -p 8080:8080 \
  -v openhere-data:/data \
  -e SECRET_KEY="$(cat /root/openhere-secret-key)" \
  openhere
```

## Operating the container

| Task | Command |
|---|---|
| Follow logs | `docker logs -f openhere` |
| Stop gracefully | `docker stop -t 30 openhere`. The server waits up to 15 seconds for requests to finish and flushes buffered analytics, and `docker stop` kills the process after 10 seconds unless you raise the timeout. |
| Upgrade | Build or pull the new image, `docker rm -f openhere`, then run again with the same volume. See [Upgrading and backups](upgrading-and-backups.md). |
| Run one server per data volume | Do not start two containers on one volume. |

The image has no shell and no health check. Probe the port with your orchestrator's TCP check.
