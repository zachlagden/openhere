# selfhost

Self-hosted, open source alternative to here.now: hosting for built static sites, agent-first, with a dashboard and a REST API. Built for one person or a small team on a VPS.

The name "selfhost" is a placeholder and will change. Keep it in as few places as possible: the `go.mod` module path, `cmd/selfhost`, the README and docs title, and the Docker image and binary name. Do not spread it into other identifiers, paths or strings.

## Stack

- Go single binary. The dashboard (Vite, React, TypeScript, Tailwind, shadcn/ui, TanStack Query, pnpm) comes later and gets embedded.
- PostgreSQL through pgx (`pgxpool`) when `DATABASE_URL` is set. SQLite through `modernc.org/sqlite` (pure Go, no CGO) at `<DATA_DIR>/selfhost.db` otherwise.
- Raw SQL with parameterised queries, no ORM. Write placeholders as `$1`, `$2`. The SQLite driver rewrites them to `?1`, `?2`.
- Keep SQL portable between both databases. No JSONB, arrays, LISTEN/NOTIFY or other PostgreSQL-only features.
- Migrations are plain SQL files in `internal/store/migrations`, embedded with `go:embed` and run by goose on startup. One migration set serves both databases.
- Logging is structured JSON through `log/slog`. Errors go to Sentry only when `SENTRY_DSN` is set.
- Layout is in `internal/`: `config`, `store` (with `storetest` for database tests), `telemetry`, `auth` (passwords, sessions, API keys, setup), `workspaces` (workspaces, grants, the authorization function), `sites` (folders, sites, listing), `publish` (manifests, versions, finalize, restore, retention, maintenance), `sitefiles` (version files on disk), `sitepath` (path safety), `settings` (typed instance settings), `ids`, `slug` and `validate` (small shared helpers), `ratelimit`, `api` (HTTP handlers and middleware). Business logic lives in plain packages so it tests without HTTP. Handlers stay thin.

## Rules

- Write no code comments. The only allowed marker is `TODO(zach): reason`.
- Handle every error explicitly. Never swallow one.
- The API documents itself in `docs/openapi.yaml`. Keep it in step with the handlers.
- Nothing is reachable without auth, except hosted sites and the login, setup and invite pages. Add no health endpoint, root page or docs route that skips auth.
- Docker Compose is forbidden, including for development. Run Postgres for tests with a plain `docker run`.
- Deploy with the Dockerfile only.
- Use only fictional placeholder values (example.com, Alex, Acme Ltd) in docs, tests and examples. Never commit a secret.
- Commits follow `<type>(<scope>): <description>`: lowercase type, imperative mood, 72 characters at most, no trailing period. Work happens on branches, not on `main`.
- Versions are `MAJOR.MINOR.PATCH`, released with annotated `vX.Y.Z` tags.
- Write prose in plain, direct English: active voice, no filler, sentence-case headings.

## Authorization

- `workspaces.EffectiveRole` is the only place that decides a role. Services load a user's grants into `workspaces.Access` and ask it. Do not re-implement the rule. The one exception is the SQL fragment `workspaces.SiteCoverSQL` for listing, which a test checks against `EffectiveRole`.
- A resource the user cannot see returns `workspaces.ErrNotFound` (404). A visible resource with too low a role returns `ErrForbidden` (403). `workspaces.Require` does both.
- Create shared workspaces only through `workspaces.Service.CreateShared`, which later gates creation. Create sites only through `sites.Service.CreateSite`, which publishing reuses.
- Pass every user-supplied string through `validate` and `slug` before it reaches SQL, and use `$n` placeholders only.

## Publishing and settings

- Every file path from a client passes `sitepath.Validate`, and every manifest passes `sitepath.CheckSet`. `sitefiles` is the only code that touches `DATA_DIR/sites`, builds paths from validated ids and paths, and opens files through `os.Root`. Never follow or create a symlink.
- A version is `pending` or `finalized`. Whether it is live comes from `sites.current_version_id`, never from a stored flag. Only `publish` changes that pointer, in one transaction that also re-checks the base version.
- A finalized version directory never changes. Upload and finalize take a per-version lock, so a file cannot land after finalize.
- Return stored files only as `application/octet-stream` with `Content-Disposition: attachment` from the API. Serving them with their real type belongs to the hosting step.
- Add an instance setting by adding a field to `settings.Settings` and one entry to `definitions` in `internal/settings/definitions.go`, then document it in `docs/openapi.yaml` and the README table.
- Add maintenance work by passing an `auth.MaintenanceTask` to `RunMaintenance` in `cmd/selfhost`.

## Commands

```sh
go build ./cmd/selfhost
go run ./cmd/selfhost
gofmt -l .
go vet ./...
golangci-lint run
go test ./...
TEST_DATABASE_URL=postgres://user:password@localhost:5432/selfhost_test go test ./...
docker build -t selfhost .
```

The store, auth and API tests always run on SQLite and run on PostgreSQL when `TEST_DATABASE_URL` is set. The store tests drop the tables they create, so point that variable at a throwaway database. The `storetest` package gives the other tests a schema of their own.
