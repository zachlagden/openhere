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
- Layout is in `internal/`: `config`, `store` (with `storetest` for database tests), `telemetry`, `auth` (passwords, sessions, API keys, setup), `workspaces` (workspaces, grants, the authorization function), `sites` (folders, sites, listing), `domains` (custom domain names, claims, verification, lookup), `publish` (manifests, versions, finalize, restore, retention, maintenance), `hosting` (host router, file serving, path-mode sandbox, wildcard check, the viewer gate), `access` (access modes, viewer cookies, email codes, sign-in handoff tokens), `mail` (the sender interface, the SMTP and Resend senders that follow the live settings, a recorder, and `smtptest`, a fake SMTP server for tests), `secrets` (AES-256-GCM sealing and the secret key), `otp` (numeric codes), `invites`, `passwordreset`, `agentcode` and `policy` (instance-wide permission settings), `proxy` (trusted proxy addresses, client IP and HTTPS detection), `certs` (built-in TLS: certmagic setup, the on-demand decision, DNS provider, the port-80 redirect, certificate status), `sitefiles` (version files on disk), `sitepath` (path safety), `settings` (typed instance settings), `ids`, `slug` and `validate` (small shared helpers), `analytics` (the view recorder, user-agent parsing, visitor hashes, rollups, retention and report queries, with `analytics/geo` for the GeoLite2 lookup and updater), `ratelimit`, `api` (HTTP handlers and middleware). Business logic lives in plain packages so it tests without HTTP. Handlers stay thin.

## Rules

- Write no code comments. The only allowed marker is `TODO(zach): reason`.
- Handle every error explicitly. Never swallow one.
- The API documents itself in `docs/openapi.yaml`. Keep it in step with the handlers.
- Nothing is reachable without auth, except the dashboard shell and its static assets on the management host (see Dashboard), which carry the login, setup, invite and password reset screens, hosted sites, the invite lookup and accept routes (`GET /api/v1/invites/{token}`, `POST /api/v1/invites/{token}/accept`), the password reset routes (`POST /api/v1/auth/password-reset/{request,confirm}`), the agent sign-in routes (`POST /api/v1/auth/agent/{request-code,verify-code}`), the wildcard probe route (`/.well-known/selfhost-check/{token}` on non-management hosts, live only during a check), the viewer gate routes of gated sites (`/.well-known/selfhost-auth` and `/.well-known/selfhost-gate/{unlock,code,verify}` on site hosts and under `/s/{slug}/`), and `GET /api/v1/access/handoff`, which redirects a caller without credentials to `/login`. The port-80 listener of built-in TLS also answers `/.well-known/acme-challenge/` through certmagic and redirects everything else to HTTPS, and it never reaches the API router. Add no health endpoint, root page or docs route that skips auth. Everything else on the management host answers 401 without credentials, or 404 with them.
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
- Add an instance setting by adding a field to `settings.Settings` and one entry to `definitions` in `internal/settings/definitions.go`, then document it in `docs/openapi.yaml` and `docs/configuration.md`.
- Add maintenance work by passing an `auth.MaintenanceTask` to `RunMaintenance` in `cmd/selfhost`.

## Hosting

- `hosting.Router` wraps the API handler and picks by request host, in this order: the host of `PUBLIC_URL` goes to the API (plus `/s/` for path mode), `{slug}.{baseDomain}` goes to the site, a verified custom domain goes to its site while `hosting.customDomainsEnabled` is on, and anything else is a 404. Site hosts must never reach the auth middleware, CSRF logic or any API route.
- Every not-found cause (unknown host, unknown slug, disabled mode, no live version) goes through `writeNotFound`, so the status, headers and body stay identical. Do not add detail to it.
- Every `/s/` response, including errors and redirects, carries the sandbox `Content-Security-Policy` without `allow-same-origin`. Subdomain mode sets no policy.
- The session cookie is `__Host-session` when the request is HTTPS (the `PUBLIC_URL` scheme or a trusted proxy) and `session` otherwise. Read only the form that fits the request.
- Code that changes a site's live version, SPA mode or existence must call the `OnSiteChange` callback of `publish` or `sites` after the change commits. `main` wires it to `hosting.Resolver.Invalidate`.
- Files are opened only through `sitefiles.Store.Open`, never by joining paths in `hosting`.
- The wildcard check writes its outcome through `settings.Service.RecordWildcard` (manual) or `RecordScheduledWildcard` (startup and scheduled). A failed check keeps subdomain mode serving, and a scheduled one adds to `hosting.wildcardFailures` while `hosting.subdomainEnabled` is on. A retry runs every 15 minutes until a check passes. After 3 consecutive failures the check switches `hosting.subdomainEnabled` off and records `hosting.subdomainDisabledReason`, only when `hosting.pathEnabled` is on. Manual checks never count. No settings write or automatic change may leave both modes off (`ErrNoHostingMode`). Enabling subdomain mode needs status `ok`.
- Per-site and instance availability come from `hosting.SiteHosting` and `hosting.InstanceHosting`. Notices are plain language with no administrator detail, and the root-only detail of `GET /api/v1/hosting/status` stays out of every other response.

## Custom domains

- Domain names pass `domains.Normalise` before they reach SQL or a comparison. Compare and store only the normalised form.
- A domain serves only while `site_domains.verified_at` is set. The router asks `domains.Service.Lookup`, which answers from a cache that every claim, verification, primary change, delete, maintenance run and `OnSiteChange` call invalidates. Never query `site_domains` from `hosting` directly.
- Verification reads TXT records at `_selfhost-challenge.<domain>` (the `domains.ChallengeLabel` constant) through the injected `domains.TXTResolver`. Do not require A, AAAA or CNAME records to point at the instance. Only a successful lookup without the token or a not-found answer counts as a miss. A timeout or resolver error never does.
- `domains.Service.Maintain` runs from the maintenance loop in `cmd/selfhost`. It releases unverified claims older than 7 days and re-checks each verified domain once a day. Three consecutive misses clear `verified_at` and the primary flag.
- Claims and primary changes lock the site row with `UPDATE sites SET id = id WHERE id = $1` inside the transaction, so the per-site limit and the one-primary rule hold on PostgreSQL. `primary_site_id` is unique, which makes a second primary fail at the database.
- The handoff `host` parameter is accepted only through `domains.Service.ServesSite`, which requires a verified domain of that same site and `hosting.customDomainsEnabled`. Build custom domain URLs only with `hosting.CustomDomainURL`.
- Domain routes sit under `/api/v1/sites/{slug}/domains` and add no unauthenticated route. Viewing needs read on the site and every change needs write.

## Access

- A site's mode, password hash, epoch and allowlists change only through `access.Service.Update`. Any change to the mode, password or a list must move the epoch and discard that site's codes and handoff tokens. Do not write those columns anywhere else.
- `hosting` asks `access.Service.Check` before it looks up any file, `404.html` included. A gated site must never answer differently for paths that exist and paths that do not. Gate pages show no site name, description or file name.
- Every response for a gated site, gate pages and plain 401s included, carries `frame-ancestors 'none'` and `X-Frame-Options: DENY`. In path mode the policy keeps the sandbox directives and adds `frame-ancestors`. Gated content is `Cache-Control: private`, gate pages `private, no-store`.
- Viewer cookies are read and written only in `hosting/gate_cookie.go`. Their names and attributes depend on subdomain or path mode and on HTTPS. Path mode over plain HTTP shows the HTTPS-required page and sets no cookie.
- The viewer signing secret lives in `instance_settings` under `internal.viewerSecret`. It is not a `settings.definitions` entry, so the settings API never returns or accepts it. Never log it, a cookie value, a code, a handoff token or a password.
- Return paths come from `access.SafeReturnPath` and nothing else. Never build a redirect from a client-supplied path without it.
- A code request answers the same for allowlisted and other addresses, including its rate limits. Do not add a branch that changes the status, body or limit behaviour by allowlist membership.

## Built-in TLS

- Built-in TLS is off by default and is switched on only by environment (`TLS_ENABLED`), because it changes the listeners. Behind a proxy it stays off and sends no HSTS.
- `certs.Policy.Allow` is the only on-demand decision. It allows the management host, and a name only when `domains.Service.Lookup` reports a verified custom domain while `hosting.customDomainsEnabled` is on. It refuses names at or under the base domain, because subdomain sites use the wildcard certificate only. Do not add an allow path, and never log or return a refused name.
- The port-80 handler is certmagic's challenge handler wrapped around `redirectHandler`. It never reaches the API router or `hosting`. The HTTPS server and the port-80 server set `ErrorLog` to a discarded logger, because handshake errors carry attacker-chosen server names.
- Certmagic logs go through `newCertmagicLogger`, which drops messages that include client-supplied names. Add any new such message to `droppedMessages`.
- The DNS token is read from settings and opened on every provider call (`liveProvider`). Never cache it or put it in an error, log line or response. Add a provider by adding a builder to `DefaultProviders` and a value to `tls.dnsProvider`.
- The wildcard certificate is requested when the base domain and a configured provider both exist, whether or not subdomain mode is on, because the wildcard check needs it first. `Manager.reconcile` applies setting changes without a restart.
- HSTS is sent only when `TLS_ENABLED` and the request arrived on the TLS listener (`r.TLS != nil`), with the value in `hosting.StrictTransportSecurity`. It has no `includeSubDomains` or `preload`.
- The Pebble tests in `internal/certs/pebble_test.go` skip unless `PEBBLE_DIRECTORY`, `PEBBLE_CA_ROOT` and `PEBBLE_DNS` are set. No test may contact a real certificate authority.

## Email, invites and sign-in

- Mail goes through `mail.Sender`. `main` wires `mail.New` over the live settings, so provider changes apply without a restart. `Configured()` is true only when the provider and its required fields are valid. Every feature that needs email checks it and degrades. Tests use `mail.Recorder`, or `smtptest` and an `httptest` server for the real senders.
- The SMTP password and the Resend API key are settings entries marked `secret`. The settings API reports `<key>Set` and never the value. `secrets.Box` seals them with AES-256-GCM, and its key comes from `SECRET_KEY` or `DATA_DIR/secret.key`. Open a secret only inside `mail.SettingsSource`.
- Send errors reach clients only as `mail.Reason(err)`. Never put a server reply, an address or a secret in a message to a client or in a log.
- Never log a code, token, reset link, invite link, API key or password. Store only hashes of tokens (`auth.HashToken`) and HMACs of codes (`secrets.Box.Sign`).
- Responses that must not reveal whether an account exists (password reset request, agent `request-code`, invite lookup and accept failures) return one fixed body for every case, and their rate limits count every request alike. The one exception is `email_not_configured`, which tells an agent to ask for a dashboard key.
- A disabled user (`users.disabled_at`) fails every session and API key lookup in the same query. Add no lookup of a user that skips `disabled_at IS NULL`.
- Every argon2 hash or verify goes through `auth.HashPassword` or `auth.VerifyPassword`, which take a slot from `hashlimit` first (`ARGON2_CONCURRENCY`, default 2). A caller that waits more than 5 seconds gets `hashlimit.ErrBusy`, which the API turns into 503 `server_busy`. Never call `argon2` directly.
- Invites create instance accounts. Create users only through `auth.Service` (`CreateUser`, `Setup`, `InsertMember`) so the personal workspace exists.

## Analytics

- Analytics is cookieless and server-side. The hosting layer counts the requests it serves, and no script is injected into a site.
- Never store the IP address, the raw User-Agent, a query string, a cookie or any header value, and never log them. Use the IP only in memory, for `geo.Lookup` and for `analytics.Hasher.Visitor`. The referrer is reduced to its host, the path to the request path without query and fragment.
- The only hook is `siteServer.countView` in `internal/hosting/views.go`. `respondWithFile` calls it after the response is written, so the access gate has already decided. Count only GET responses with an HTML content type and status 200 or 404, and skip Range, 304, prefetch and HEAD. The client address comes from `proxy.Trust.ClientIP`.
- Recording is asynchronous and never blocks or fails a response. `analytics.Recorder.Record` is a non-blocking send into a bounded buffer and drops the event when the buffer is full. Do not add a synchronous database write or a lock on the request path. `Recorder.Close` flushes the buffer at shutdown.
- A site records only while `sites.analytics_enabled` is true. `hosting.Resolver` carries the flag, and `OnSiteChange` refreshes it.
- Reports read rollups for days that have an `events` marker row in `analytics_daily` and raw events for the rest. Change one query, change both sides, and keep `Rollup` idempotent.
- The visitor secret lives sealed in `instance_settings` under `internal.analyticsSecret`. It is not a `settings.definitions` entry, so the settings API never returns it. The MaxMind licence key is a sealed secret setting. Open it only in `geo.Updater` and never log it.
- The GeoLite2 updater (`internal/analytics/geo`) writes the database to `DATA_DIR/geo` through a temporary file and an atomic rename. Tests never contact MaxMind: use a local TLS `httptest` server and a database built with `mmdbwriter`.

## Dashboard

- The dashboard is a single page app in `web/` (Vite, React, TypeScript, Tailwind, react-router, TanStack Query). `pnpm --dir web build` writes it to `internal/ui/bundle`, which `go:embed` compiles into the binary. The built files are not committed. `internal/ui/bundle/.gitkeep` keeps the embed directive compiling on a fresh clone, and without a build the shell routes answer 503 with a plain "dashboard not built" text.
- `ui.Shell.Wrap` goes around the API handler, inside `hosting.Router`, so only the management host can serve it. It serves the shell for every GET and HEAD path except `/api/`, `/s/` and `/.well-known/`, serves files from the build at their own path, and answers 404 JSON for a missing path with a file extension. Site hosts never reach it.
- This is the one deliberate exception to "nothing without auth". The shell and its hashed assets must hold nothing specific to the instance: no site names, users, settings or inline secrets. Keep scripts out of the HTML (`script-src 'self'`), and keep the strict Content-Security-Policy in `internal/ui/shell.go`.
- Hashed files under `assets/` are `public, max-age=31536000, immutable`. Every other file, the shell included, is `no-cache`.
- The dashboard never uploads or publishes sites. Agents do that through the API. Everything else it does goes through the documented API, with `X-Requested-With` on state-changing requests.
- `web/src/api/schema.d.ts` is generated from `docs/openapi.yaml` by `pnpm --dir web gen:api`. Regenerate and commit it whenever the spec changes.
- The product name appears in the dashboard only in `web/src/brand.ts` and `web/index.html`.
- Checks: `pnpm --dir web lint`, `typecheck`, `test`, `build`. `pnpm --dir web e2e` drives a built binary in a real browser.

## Dashboard

- The dashboard has no upload or publish screen. Agents publish through the API.
- `/admin` is for the root user only. Everyone else gets the same not-found page as for an unknown route, and the admin entry stays out of their menu.
- Draw charts as plain SVG. The Content-Security-Policy forbids inline styles and scripts, so use no chart library that injects them, and use native `<dialog>` elements for dialogs.
- Show a one-time secret (API key, invite link, reset link) only through `SecretDialog`, and call `reset()` on the mutation once the value has been handed over. Never render a stored secret. Show whether it is set.
- Per-call `mutate` callbacks do not run after the calling component unmounts. Do not remount a form with a changing `key` after a save, and use `mutateAsync` where success removes the screen, as the delete flows do.
- Build instance settings forms with `SettingsForm` and the `Setting*` fields. They send only the changed keys, map the API's validation message to its field by the setting key in the message, and ask for confirmation through `destructiveChanges`.
- Keep rules in `web/src/lib` as plain functions with tests (ranges, chart scales, allowlists, member rules, settings diffs) and keep screens thin.
- Set the browser tab title with `usePageTitle` on every page.

## Commands

```sh
go build ./cmd/selfhost
go run ./cmd/selfhost
gofmt -l .
go vet ./...
golangci-lint run
go test ./...
TEST_DATABASE_URL=postgres://user:password@localhost:5432/selfhost_test go test ./...
pnpm --dir web install
pnpm --dir web build
docker build -t selfhost .
```

The store, auth and API tests always run on SQLite and run on PostgreSQL when `TEST_DATABASE_URL` is set. The store tests drop the tables they create, so point that variable at a throwaway database. The `storetest` package gives the other tests a schema of their own.
