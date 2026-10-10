# Changelog

All notable changes to this project are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

The first release has not been cut. This section describes what the software does today.

### Added

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
