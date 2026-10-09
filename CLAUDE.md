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
- Layout is in `internal/`: `config`, `store`, `telemetry`. Business logic lives in plain packages so it tests without HTTP. Handlers stay thin.

## Rules

- Write no code comments. The only allowed marker is `TODO(zach): reason`.
- Handle every error explicitly. Never swallow one.
- Nothing is reachable without auth, except hosted sites and the login, setup and invite pages. Add no health endpoint, root page or docs route that skips auth.
- Docker Compose is forbidden, including for development. Run Postgres for tests with a plain `docker run`.
- Deploy with the Dockerfile only.
- Use only fictional placeholder values (example.com, Alex, Acme Ltd) in docs, tests and examples. Never commit a secret.
- Commits follow `<type>(<scope>): <description>`: lowercase type, imperative mood, 72 characters at most, no trailing period. Work happens on branches, not on `main`.
- Versions are `MAJOR.MINOR.PATCH`, released with annotated `vX.Y.Z` tags.
- Write prose in plain, direct English: active voice, no filler, sentence-case headings.

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

The store tests always run on SQLite and run on PostgreSQL when `TEST_DATABASE_URL` is set. They drop the tables they create, so point that variable at a throwaway database.
