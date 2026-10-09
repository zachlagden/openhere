# selfhost

Self-hosted, open source alternative to here.now: hosting for built static sites, agent-first, with a dashboard and a REST API. Built for one person or a small team on a VPS.

The name "selfhost" is a placeholder and will change. Keep it in as few places as possible: the `go.mod` module path, `cmd/selfhost`, the README and docs title, the Docker image and binary name, and the agent credentials directory in `internal/config/agent.go`. Do not spread it into other identifiers, paths or strings.

## Stack

- Go single binary. The dashboard (Vite, React, TypeScript, Tailwind, shadcn/ui, TanStack Query, pnpm) comes later and gets embedded.
- PostgreSQL through pgx (`pgxpool`) when `DATABASE_URL` is set. SQLite through `modernc.org/sqlite` (pure Go, no CGO) at `<DATA_DIR>/selfhost.db` otherwise.
- Raw SQL with parameterised queries, no ORM. Write placeholders as `$1`, `$2`. The SQLite driver rewrites them to `?1`, `?2`.
- Keep SQL portable between both databases. No JSONB, arrays, LISTEN/NOTIFY or other PostgreSQL-only features.
- Migrations are plain SQL files in `internal/store/migrations`, embedded with `go:embed` and run by goose on startup. One migration set serves both databases.
- Logging is structured JSON through `log/slog`. Errors go to Sentry only when `SENTRY_DSN` is set.
- Layout is in `internal/`: `config`, `store` (with `storetest` for database tests), `telemetry`, `auth` (passwords, sessions, API keys, setup), `workspaces` (workspaces, grants, the authorization function), `sites` (folders, sites, listing), `publish` (manifests, versions, finalize, restore, retention, maintenance), `hosting` (host router, file serving, path-mode sandbox, wildcard check, the viewer gate), `access` (access modes, viewer cookies, email codes, sign-in handoff tokens), `mail` (the sender interface, the SMTP and Resend senders that follow the live settings, a recorder, and `smtptest`, a fake SMTP server for tests), `secrets` (AES-256-GCM sealing and the secret key), `otp` (numeric codes), `invites`, `passwordreset`, `agentcode` and `policy` (instance-wide permission settings), `proxy` (trusted proxy addresses, client IP and HTTPS detection), `sitefiles` (version files on disk), `sitepath` (path safety), `settings` (typed instance settings), `ids`, `slug` and `validate` (small shared helpers), `ratelimit`, `api` (HTTP handlers and middleware). Business logic lives in plain packages so it tests without HTTP. Handlers stay thin.

## Rules

- Write no code comments. The only allowed marker is `TODO(zach): reason`.
- Handle every error explicitly. Never swallow one.
- The API documents itself in `docs/openapi.yaml`. Keep it in step with the handlers.
- Nothing is reachable without auth, except hosted sites, the login, setup, invite and password reset pages, the invite lookup and accept routes (`GET /api/v1/invites/{token}`, `POST /api/v1/invites/{token}/accept`), the password reset routes (`POST /api/v1/auth/password-reset/{request,confirm}`), the agent sign-in routes (`POST /api/v1/auth/agent/{request-code,verify-code}`), the wildcard probe route (`/.well-known/selfhost-check/{token}` on non-management hosts, live only during a check), the viewer gate routes of gated sites (`/.well-known/selfhost-auth` and `/.well-known/selfhost-gate/{unlock,code,verify}` on site hosts and under `/s/{slug}/`), and `GET /api/v1/access/handoff`, which redirects a caller without credentials to `/login`. Add no health endpoint, root page or docs route that skips auth.
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
- Return stored files only as `application/octet-stream` with `Content-Disposition: attachment` from the API. Only `hosting` serves them with their real type.
- Add an instance setting by adding a field to `settings.Settings` and one entry to `definitions` in `internal/settings/definitions.go`, then document it in `docs/openapi.yaml` and the README table.
- Add maintenance work by passing an `auth.MaintenanceTask` to `RunMaintenance` in `cmd/selfhost`.

## Hosting

- `hosting.Router` wraps the API handler and picks by request host: the host of `PUBLIC_URL` goes to the API (plus `/s/` for path mode), `{slug}.{baseDomain}` goes to the site, and anything else is a 404. Site hosts must never reach the auth middleware, CSRF logic or any API route.
- Every not-found cause (unknown host, unknown slug, disabled mode, no live version) goes through `writeNotFound`, so the status, headers and body stay identical. Do not add detail to it.
- Every `/s/` response, including errors and redirects, carries the sandbox `Content-Security-Policy` without `allow-same-origin`. Subdomain mode sets no policy.
- The session cookie is `__Host-session` when the request is HTTPS (the `PUBLIC_URL` scheme or a trusted proxy) and `session` otherwise. Read only the form that fits the request.
- Code that changes a site's live version, SPA mode or existence must call the `OnSiteChange` callback of `publish` or `sites` after the change commits. `main` wires it to `hosting.Resolver.Invalidate`.
- Files are opened only through `sitefiles.Store.Open`, never by joining paths in `hosting`.
- The wildcard check writes its outcome through `settings.Service.RecordWildcard`. A failed check switches `hosting.subdomainEnabled` off, and enabling it needs status `ok`.

## Access

- A site's mode, password hash, epoch and allowlists change only through `access.Service.Update`. Any change to the mode, password or a list must move the epoch and discard that site's codes and handoff tokens. Do not write those columns anywhere else.
- `hosting` asks `access.Service.Check` before it looks up any file, `404.html` included. A gated site must never answer differently for paths that exist and paths that do not. Gate pages show no site name, description or file name.
- Every response for a gated site, gate pages and plain 401s included, carries `frame-ancestors 'none'` and `X-Frame-Options: DENY`. In path mode the policy keeps the sandbox directives and adds `frame-ancestors`. Gated content is `Cache-Control: private`, gate pages `private, no-store`.
- Viewer cookies are read and written only in `hosting/gate_cookie.go`. Their names and attributes depend on subdomain or path mode and on HTTPS. Path mode over plain HTTP shows the HTTPS-required page and sets no cookie.
- The viewer signing secret lives in `instance_settings` under `internal.viewerSecret`. It is not a `settings.definitions` entry, so the settings API never returns or accepts it. Never log it, a cookie value, a code, a handoff token or a password.
- Return paths come from `access.SafeReturnPath` and nothing else. Never build a redirect from a client-supplied path without it.
- A code request answers the same for allowlisted and other addresses, including its rate limits. Do not add a branch that changes the status, body or limit behaviour by allowlist membership.

## Email, invites and sign-in

- Mail goes through `mail.Sender`. `main` wires `mail.New` over the live settings, so provider changes apply without a restart. `Configured()` is true only when the provider and its required fields are valid. Every feature that needs email checks it and degrades. Tests use `mail.Recorder`, or `smtptest` and an `httptest` server for the real senders.
- The SMTP password and the Resend API key are settings entries marked `secret`. The settings API reports `<key>Set` and never the value. `secrets.Box` seals them with AES-256-GCM, and its key comes from `SECRET_KEY` or `DATA_DIR/secret.key`. Open a secret only inside `mail.SettingsSource`.
- Send errors reach clients only as `mail.Reason(err)`. Never put a server reply, an address or a secret in a message to a client or in a log.
- Never log a code, token, reset link, invite link, API key or password. Store only hashes of tokens (`auth.HashToken`) and HMACs of codes (`secrets.Box.Sign`).
- Responses that must not reveal whether an account exists (password reset request, agent `request-code`, invite lookup and accept failures) return one fixed body for every case, and their rate limits count every request alike. The one exception is `email_not_configured`, which tells an agent to ask for a dashboard key.
- A disabled user (`users.disabled_at`) fails every session and API key lookup in the same query. Add no lookup of a user that skips `disabled_at IS NULL`.
- Invites create instance accounts. Create users only through `auth.Service` (`CreateUser`, `Setup`, `InsertMember`) so the personal workspace exists.

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
