# Changelog

All notable changes to this project are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

The first release has not been cut. This section describes what the software does today.

### Added

- Saving MaxMind credentials checks the GeoLite2 database within seconds, a restart retries a failure older than 5 minutes, and the root user can run a check from the dashboard (Check now) or with `POST /api/v1/admin/analytics/geo/check`, at most once a minute. A failed download no longer waits 6 hours after you fix the cause, and a rejected new key now says it may not be active yet.
- `CLIENT_IP_HEADER` setting. Behind Cloudflare and a proxy that overwrites `X-Forwarded-For`, it names the header (for example `CF-Connecting-IP`) that carries the real client address. The server reads it only from `TRUSTED_PROXIES` peers, so rate limits, login lockouts, gate limits, analytics visitor hashes and location lookups key on the client and not on a Cloudflare edge address.
- A single Go binary that serves a REST API, a web dashboard and the hosted sites. It runs on SQLite by default and on PostgreSQL when `DATABASE_URL` is set. Migrations are embedded and run on start.
- First-run setup: the server logs a one-time setup token while the database has no users, and the first user becomes the root user.
- Accounts with sessions for browsers and API keys for scripts and agents, invites by link or email, password resets, and an email-code flow that lets an agent obtain an API key.
- Workspaces with `admin`, `write` and `read` roles, grants scoped to a whole workspace, one folder or one site, flat folders, and a personal workspace for every user.
- Publishing in three steps (describe the files, upload each file, finalize), with SHA-256 and size checks, atomic go-live, version history, restore, conflict detection with a base version, retention and single-page app mode.
- Hosting at `{slug}.{baseDomain}` (subdomain mode, behind a wildcard DNS check), at `{PUBLIC_URL}/s/{slug}/` (path mode, sandboxed) and at custom domains verified with a DNS TXT record.
- Per-site access modes: public, password, and restricted to instance users plus email and domain allowlists, with email codes for allowlisted viewers.
- Optional built-in HTTPS with ACME certificates, on-demand issuance for verified custom domains and a Cloudflare DNS-01 wildcard certificate for subdomain mode.
- Cookieless page view analytics with unique visitor counts, daily rollups, retention settings and optional country and city lookups from a MaxMind GeoLite2 database.
- Instance settings for retention, limits, hosting, access policy, email (SMTP or Resend), certificates and analytics, with write-only encrypted secrets.
- An OpenAPI 3.1 description of every route, and an agent guide, both embedded in the binary and served to signed-in users.
- A dashboard for sites, versions, access, domains, analytics, workspaces, members, invites, API keys and instance administration.
- A multi-architecture `Dockerfile`, and a release workflow that builds archives, a container image and a GitHub release from an annotated `vX.Y.Z` tag.
- Optional error reporting through the Sentry SDK when `SENTRY_DSN` is set, and structured JSON logs.

### Fixed

- Copy buttons in the dashboard confirm a copy on the button itself. The icon changes to a tick for 2 seconds and the accessible name changes to "Copied", with no toast. A failed copy shows a cross and "Copy failed", and the one-time key and link dialogs select the value so it can be copied by hand.

### Security

- Password hashing is limited to 2 hashes at once (`ARGON2_CONCURRENCY`, 1 to 16). A burst of sign-in requests can no longer exhaust memory. A request that waits more than 5 seconds gets 503 `server_busy`.
- The server no longer stores the signing secret for viewer cookies of gated sites. It derives the keys from the sealing key, and a migration deletes the old stored secret. Every viewer of a gated site has to sign in again after the upgrade.
- Pending versions are capped at 3 per site, 10 per user and `limits.maxPendingBytes` (4 GiB) in total. A manifest over a cap gets 429 `too_many_pending`.
- Analytics count a view only for a page that exists in the live version. Unknown paths share one bucket, each site stores at most 1000 distinct paths a day, and one visitor can add at most 120 views a minute.
- Only a workspace-wide `admin` deletes a folder, and only a workspace-wide `write` or `admin` renames one. A grant limited to a folder no longer allows either. Deleting a folder that still has folder grants returns 409 `folder_has_grants` unless the request adds `force=true`.
- Every listener now closes slow or idle connections: 10 seconds for headers, 30 seconds for the request, 60 seconds for the response and 90 seconds idle. Uploads and file downloads get more time in proportion to their size.
- The build uses Go 1.26.9, which fixes the standard library vulnerabilities that `govulncheck` reported for 1.26.4. CI now runs `govulncheck`.
- Failed sign-ins count per address and email pair, so a stranger can no longer lock someone out by guessing their email address. A flood of failures for one email slows only the addresses that already failed for it.
- A site that verifies a custom domain with its TXT record displaces other sites' unverified claims on the same name. Claiming a name no longer shows who holds it.
- Certificate issuance on demand counts only requests that reach the certificate authority, and one site may use at most 10 of the 50 hourly requests.
- Sign-in code emails for gated sites have an instance-wide limit of 100 per hour (`limits.maxGateCodeMailsPerHour`).
- The data directory is created with mode 0700 and the SQLite files with mode 0600. The server tightens existing ones at start.
- The rate limiter drops the oldest windows first when its key table is full.
- Sessions end 90 days after sign-in at the latest. A password reset can also revoke API keys with `revokeApiKeys`.
- Server error logs name the route pattern and no longer include the request path, which could hold an invite token. The server refuses to send an SMTP password over an unencrypted connection to a host other than `localhost` or a loopback address.
- The Docker build context leaves out `.env` files, data directories, databases, keys and certificates.
- New sites cannot use more infrastructure names such as `mta-sts`, `autodiscover` and `vpn`, or a slug that starts with `xn--`.
