# selfhost

A self-hosted, open source alternative to here.now: hosting for already built static sites, aimed at AI agents, with a dashboard and a REST API. It targets one person or a small team running it on a VPS. It is not a public sign-up service.

"selfhost" is a placeholder name and will change.

## Status

Early development. The server starts, loads its configuration, connects to the database, applies migrations and serves the authentication API (first-time setup, login, sessions and API keys) and the workspace API (workspaces, members, folders and empty sites). Sites hold no files yet, so there is no publishing and no hosting. There is no dashboard.

## First-time setup

A new instance has no users, so whoever reaches it first could claim it. To prevent that, the server generates a random setup token at startup while the database has no users. It keeps the token in memory and logs it once at info level, together with the setup path:

```json
{"level":"INFO","msg":"setup required: send this token with the first setup request","setup_path":"/api/v1/setup","setup_token":"..."}
```

Create the root user by sending that token with the first request. The server discards the token after setup, and a restart on a configured instance logs no token.

```sh
curl -X POST http://localhost:8080/api/v1/setup \
  -H 'Content-Type: application/json' \
  -d '{"email":"alex@example.com","password":"a password of 12 or more characters","setupToken":"<token from the log>"}'
```

Passwords need 12 to 128 characters. Emails are ASCII only and stored in lower case.

## API

`docs/openapi.yaml` describes every endpoint (OpenAPI 3.1). The server does not serve it yet. Only `GET /api/v1/setup/status`, `POST /api/v1/setup` and `POST /api/v1/auth/login` work without credentials. Every other path returns a JSON 401, or a JSON 404 once signed in.

- Browsers sign in with a session cookie (HttpOnly, SameSite=Lax, Secure over HTTPS, 30 days, renewed on use). State-changing requests that use the cookie must send an `X-Requested-With` header and an `Origin` header that matches the host.
- Scripts and agents use `Authorization: Bearer <key>` with a key from `POST /api/v1/me/keys`. The response shows the full key once. The server stores only its SHA-256 hash. Each user can hold 50 active keys.
- Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. `retry_after` appears on 429 responses.

### Rate limits

Login and setup are limited per IP address and per email: login allows 30 attempts per IP and 10 per email in 15 minutes, setup allows 10 per IP and 5 per email. The counters live in process memory, so a restart resets them and each server process counts separately. A successful login resets that email's counter.

### Running behind a reverse proxy

Set `TRUSTED_PROXIES` to the addresses of your proxy. The server then reads the client address from `X-Forwarded-For` and decides whether to set the Secure cookie flag from `X-Forwarded-Proto`, but only for requests that arrive from those addresses. Without it, every client appears to be the proxy and shares one rate limit. The proxy must pass the original `Host` header, because the CSRF check compares it with `Origin`.

## Workspaces and sites

Every user has a personal workspace, created together with the account. It has one member, its owner, and cannot be deleted or shared. Any signed-in user can create shared workspaces with `POST /api/v1/workspaces` and becomes their admin.

A grant gives a user a role in a workspace. The roles are:

- `admin` manages members and grants, renames and deletes the workspace. Admin covers the whole workspace only.
- `write` creates, edits, moves and deletes the sites and folders it covers.
- `read` views and searches.

A grant covers the whole workspace, one folder (and the sites in it) or one site. A user can hold several scoped grants in one workspace, and the highest role among the grants that cover a resource applies. A user with only scoped grants sees only the covered folders and sites, and the workspace still appears in their list. A write grant on one folder lets the user create sites in that folder and nowhere else.

A resource the user cannot see returns 404. A resource the user can see but may not change returns 403. A workspace always keeps at least one workspace-wide admin: the last one cannot be removed or demoted. Add a member by the email of an existing account with `POST /api/v1/workspaces/{workspace}/members`. Email invites do not exist yet.

Folders are flat and belong to one workspace. Names are unique per workspace regardless of case. Deleting a folder moves its sites to unfiled.

A site belongs to one workspace and at most one folder. Its slug is unique across the instance, uses 1 to 63 lowercase letters, digits and hyphens with no leading or trailing hyphen, and cannot be a reserved word such as `www`, `api` or `admin`. The server generates a slug such as `quiet-meadow-k3p7` when you give none. The slug cannot change. `POST /api/v1/workspaces/{workspace}/sites` creates an empty site. A workspace that contains sites cannot be deleted.

`GET /api/v1/sites` lists every site the caller can reach, newest first. Filter with `workspace`, `folder` (a folder id, or `none` for unfiled sites) and `q`, which matches the display name and slug without regard to case. Page with `limit` (default 50, at most 200) and the `cursor` from the previous response's `nextCursor`.

## Configuration

The server reads these environment variables.

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Address the HTTP server listens on, as `host:port` or `:port`. |
| `DATA_DIR` | `./data` (`/data` in the Docker image) | Directory for the SQLite database file `selfhost.db`. |
| `DATABASE_URL` | unset | PostgreSQL connection URL. When unset, the server uses SQLite in `DATA_DIR`. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. Logs are JSON on standard output. |
| `SENTRY_DSN` | unset | Enables error reporting through the Sentry SDK. A self-hosted GlitchTip works. |
| `API_KEY_PREFIX` | `sk_` | Constant prefix of generated API keys, 1 to 16 letters, digits or underscores. Changing it invalidates existing keys. |
| `TRUSTED_PROXIES` | unset | Comma-separated IP addresses or CIDR ranges of reverse proxies. The server reads `X-Forwarded-For` and `X-Forwarded-Proto` only from these peers. |

`.env.example` lists the same keys with fake values. The server does not load `.env` files itself, so export the variables or pass them to Docker.

## Run with Docker

```sh
docker build -t selfhost .
docker run -p 8080:8080 -v selfhost-data:/data selfhost
```

To use PostgreSQL, add `-e DATABASE_URL=postgres://user:password@host:5432/dbname`.

## Run from source

You need the Go version in `go.mod`.

```sh
go run ./cmd/selfhost
```

## Development

```sh
gofmt -l .
go vet ./...
golangci-lint run
go test ./...
```

`go test` runs the database tests against SQLite. To run them against PostgreSQL as well, set `TEST_DATABASE_URL` to a test database. The store tests drop the tables they create. The auth and API tests create a schema of their own per test and drop it afterwards.

## Licence

MIT. See `LICENSE`.
