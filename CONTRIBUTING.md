# Contributing

Thank you for helping. This project is a self-hosted static site host for one person or a small team. Read [docs/security.md](docs/security.md) and the project rules below before you open a pull request, because several areas have strict rules.

## Set up

You need the Go version named in `go.mod`, Node 22 with pnpm (`corepack enable` provides the pinned version), and Docker for the PostgreSQL tests and the image build.

```sh
git clone https://github.com/zachlagden/openhere
cd openhere
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go build -o /tmp/openhere ./cmd/openhere
```

Build the dashboard before the server, because the server embeds the output of `pnpm --dir web build` from `internal/ui/bundle`. For dashboard development, run the server, then run `pnpm --dir web dev`, which proxies `/api` to `127.0.0.1:8080`.

## Check your work

Run these before you push. CI runs the same checks.

```sh
gofmt -l .
go vet ./...
golangci-lint run
go test -race ./...
pnpm --dir web lint
pnpm --dir web typecheck
pnpm --dir web test
pnpm --dir web build
docker build -t openhere:dev .
```

`gofmt -l .` must print nothing. Install golangci-lint from its documentation, or run it with `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run`.

### Tests on both databases

`go test` runs the database tests on SQLite. Set `TEST_DATABASE_URL` to run them on PostgreSQL as well. The store tests drop the tables they create, so point the variable at a throwaway database. The auth and API tests create a schema of their own per test and drop it afterwards.

```sh
docker run -d --rm --name openhere-test-pg -p 5432:5432 \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=openhere_test \
  postgres:17-alpine
TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/openhere_test go test -race ./...
docker stop openhere-test-pg
```

Both runs must pass. Keep SQL portable between the two databases: use `$1`, `$2` placeholders, no JSONB, arrays or LISTEN/NOTIFY.

### Dashboard end-to-end tests

`pnpm --dir web e2e` builds the server, starts it on a temporary data directory and drives it in Chrome through setup, publishing and every screen, at desktop and phone widths in light and dark. It needs `playwright-core` on `NODE_PATH` (for example from a global pnpm install) and Chrome at `CHROME_PATH` (default `/usr/bin/google-chrome`). Set `SELFHOST_BIN` to use a prebuilt binary. Screenshots go to `web/e2e/output`, which Git ignores.

### Certificate tests

The tests in `internal/certs` run against Pebble, a small ACME test server, when `PEBBLE_DIRECTORY`, `PEBBLE_CA_ROOT` and `PEBBLE_DNS` are set, and skip otherwise. Start Pebble with:

```sh
docker run --rm --network host -e PEBBLE_VA_NOSLEEP=1 \
  ghcr.io/letsencrypt/pebble:latest -config /test/config/pebble-config.json -dnsserver 127.0.0.1:8054
```

Copy `/test/certs/pebble.minica.pem` out of the image and point `PEBBLE_CA_ROOT` at it. `PEBBLE_DIRECTORY` is `https://localhost:14000/dir` and `PEBBLE_DNS` is `127.0.0.1:8054`. The tests use ports 5001 and 5002.

## Code rules

- **No code comments.** Write names and structure that explain themselves. The one allowed marker is `TODO(zach): reason`. This applies to Go, TypeScript and workflow files.
- **Handle every error explicitly.** Never swallow one.
- **Keep business logic in plain packages** under `internal/`, so it tests without HTTP. Handlers stay thin.
- **Parameterised SQL only**, written as `$1`, `$2`, with no ORM. Pass user input through `validate` and `slug` first.
- **Docker Compose is not allowed**, including for development. The deployment path is the `Dockerfile`. Run PostgreSQL for tests with a plain `docker run`.
- **Keep the API description current.** `docs/openapi.yaml` documents every route, and a test fails when a handler and the spec disagree. After you change the spec, run `pnpm --dir web gen:api` and commit the regenerated `web/src/api/schema.d.ts`. CI fails when it is stale.
- **Keep the agent guide current.** `docs/agent-guide.md` is embedded in the binary.
- **Add an instance setting** by adding a field to `settings.Settings` and an entry to `definitions` in `internal/settings/definitions.go`, then document it in `docs/openapi.yaml` and `docs/configuration.md`.
- **Do not add an unauthenticated route** without discussing it first. [docs/security.md](docs/security.md) lists the ones that exist.
- **Use fictional values** in docs, tests and examples: `example.com`, `Alex`, `Acme Ltd`, documentation IP ranges. Never commit a secret.

`CLAUDE.md` holds the detailed architecture rules for the authorization function, publishing, hosting, access control and email. Read the sections that cover the code you change.

## Commits, branches and pull requests

- Write commits as `<type>(<scope>): <description>` with type `feat`, `fix`, `docs`, `chore`, `refactor`, `style`, `test`, `perf`, `ci` or `build`. Use a lowercase type and the imperative mood, at most 72 characters, no trailing period. Make one logical change per commit. Examples: `fix(publish): reject duplicate paths`, `docs(api): describe the domain routes`.
- Work on a branch named `<type>/<short-description>`, such as `feature/domain-limits` or `fix/gate-redirect`. Do not commit to `main`.
- Give the pull request a conventional-commit title and a body that says what changed and why.
- Merge only when the local checks and CI pass.

## Documentation style

Write plain, direct English in the active voice. Use sentence-case headings and straight quotes. Say what the software does today and verify it against the code. Cut filler, vague claims and em dashes.

## Releases

Maintainers cut releases. The version is `MAJOR.MINOR.PATCH`. Move the `[Unreleased]` section of `CHANGELOG.md` under the new version heading with its date, commit it, and push an annotated tag `vX.Y.Z` on that commit. The release workflow refuses a tag that is not annotated, builds archives for four platforms, a multi-architecture image on GitHub Container Registry and the GitHub release, and uses the changelog section as the notes.

The release workflow pins every action to a commit SHA. When you update one, look up the SHA of the new tag, and record the pairs here:

| Action | Version |
|---|---|
| `actions/checkout` | v4.4.0 |
| `actions/setup-go` | v5.6.0 |
| `actions/setup-node` | v4.4.0 |
| `actions/upload-artifact` | v7.0.2 |
| `actions/download-artifact` | v8.0.2 |
| `pnpm/action-setup` | v4.4.0 |
| `docker/setup-qemu-action` | v4.4.0 |
| `docker/setup-buildx-action` | v4.4.1 |
| `docker/login-action` | v4.6.0 |
| `docker/build-push-action` | v7.4.0 |

## The project name

The name is defined in the constants listed in `CLAUDE.md`. Take it from those constants and do not spread new string literals of it through the code.
