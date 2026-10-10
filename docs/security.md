# Security

This document describes what the software protects, how, and where its limits are. To report a vulnerability, see [SECURITY.md](../SECURITY.md).

## Who this is built for

The server is for one person or a small team that trusts its own users. It has no public sign-up. Accounts exist because the root user created them or invited them. A user with write access to a site can publish any file, including JavaScript, to it. The isolation described below limits what that JavaScript can do to the dashboard and to other sites. It is not a defence against a malicious account holder abusing your server's bandwidth, storage or reputation.

## What is public

Without credentials, the server answers only:

- hosted sites that are in `public` access mode, and the gate pages of sites that are not.
- the dashboard shell and its static assets on the management host. They carry the sign-in, setup, invite and password reset screens and hold nothing about your instance.
- `GET /api/v1/setup/status` and `POST /api/v1/setup` (the latter needs the setup token), `POST /api/v1/auth/login`.
- the invite lookup and accept routes, the password reset routes and the agent sign-in routes (`request-code`, `verify-code`). Each is rate limited and answers the same way for every case.
- `GET /api/v1/access/handoff`, which redirects a caller without credentials to `/login`.
- the gate routes of gated sites: `/.well-known/selfhost-auth` and `/.well-known/selfhost-gate/{unlock,code,verify}`, on site hosts and under `/s/{slug}/`.
- while a check runs, the wildcard probe `/.well-known/selfhost-check/{token}` on site hosts. It accepts each token once within 60 seconds.
- with built-in HTTPS, the ACME challenge path on port 80, answered only for tokens the server issued.

Every other path under `/api/` returns a JSON 401, or a JSON 404 once signed in. The server has no health endpoint, no root page that skips authentication and no public API documentation. The OpenAPI document and the agent guide are served only to signed-in users, at `GET /api/v1/docs/openapi.yaml` and `GET /api/v1/docs/agent-guide.md`. If your orchestrator needs a health check, use a TCP check on the port or an authenticated request.

**A new site is public.** Anyone who knows or guesses its slug can read its files until you set another access mode or change the default with `sites.defaultAccessMode`. Generated slugs such as `quiet-meadow-k3p7` are hard to guess, and a chosen slug is not.

## Authentication

- Browsers sign in with a session cookie: `HttpOnly`, `SameSite=Lax`, 30 days, renewed on use, and never longer than 90 days after sign-in. Over HTTPS it is `__Host-session` with `Secure`, `Path=/` and no `Domain`, so a sibling subdomain cannot overwrite it. Over plain HTTP, meant for local use, it is `session`. The server reads only the form that matches the request.
- A state-changing request that uses the cookie must carry an `X-Requested-With` header and an `Origin` header whose host equals the request host. Requests with a bearer key skip this check because the browser does not attach a bearer key by itself.
- Scripts and agents send `Authorization: Bearer <key>`. The server stores only the SHA-256 hash of a key, and shows the full key once. Each user can hold 50 active keys. A key has the full access of its user.
- Passwords need 12 to 128 characters and are stored as argon2id hashes. Site passwords need 8 to 128 characters, and the same hash applies. Each hash takes 64 MiB, so the server computes at most `ARGON2_CONCURRENCY` (default 2) at once. A request that waits more than 5 seconds for a slot gets 503 `server_busy`.
- A disabled user fails every session and key lookup immediately.
- Tokens for invites, password resets and sign-in handoffs are stored as hashes. Email codes are stored as HMACs.

## Rate limits

The limits below count in process memory. A restart resets every counter, and two server processes would count separately. Limited requests get 429 with `retry_after`.

| Route | Limit |
|---|---|
| Login | 30 attempts per IP in 15 minutes. 10 failed attempts per IP and email pair in 15 minutes, reset by a successful login. After 100 failed attempts for one email in 15 minutes from any addresses, only addresses that already failed for that email get 429. A first attempt from a new address still reaches the password check. |
| Setup | 10 per IP and 5 per email in 15 minutes |
| Invite lookup | 60 per minute per IP |
| Invite accept | 10 per 15 minutes per IP |
| Create or resend invites | 30 per hour per user |
| Password reset request | 5 per 15 minutes per IP and 3 per hour per email |
| Password reset confirm | 10 per 15 minutes per IP |
| Agent code request | 10 per 15 minutes per IP, 3 per hour per email and 100 per hour for the instance |
| Agent code verify | 20 per 15 minutes per IP and 10 per 15 minutes per email |
| Account handoff | 30 per minute per user |
| Custom domain check | 20 per minute per user |
| Custom domain claim, change or delete | 30 per minute per user |
| Wildcard check (root) | 6 per minute |
| Email test (root) | 5 per minute |
| Site password unlock | 10 per IP and 100 per site in 15 minutes |
| Site email code request | 3 per address in 10 minutes, 10 per IP in 15 minutes, 60 per site per hour |
| Site email code emails | 100 per hour for the whole instance (`limits.maxGateCodeMailsPerHour`), on top of the limits above. A capped request gets the usual answer and no email goes out. |
| Site email code check | 10 per address, 20 per IP and 200 per site in 15 minutes |
| Certificates on demand | 5 per name, 10 per site and 50 in total per hour, counting only requests that reach the certificate authority |

Publishing is limited by quota instead of rate. A site may hold 3 pending versions, a user 10, and all pending versions together may declare `limits.maxPendingBytes` (4 GiB by default). A manifest over a limit gets 429 `too_many_pending`. Pending versions expire after 24 hours.

Authenticated API calls outside this table, such as publishing and listing, have no rate limit. Put a limit on the proxy if you need one.

The server tracks up to 50,000 limit keys at a time. When that many are in use, it first drops expired ones, then the 5% with the oldest windows. A flood of unique keys can therefore reset the oldest counters but never a newer one. The client address for these limits comes from `X-Forwarded-For` only when the connection comes from a `TRUSTED_PROXIES` address. Without that setting, every client behind a proxy looks like the proxy and shares one limit. An IPv6 client counts by its /64 prefix.

## Connection timeouts

Every listener, including the built-in HTTPS ones and the port 80 redirect, closes a connection that sends its request headers slower than 10 seconds, its body slower than 30 seconds in total, or sits idle longer than 90 seconds. A response must finish within 60 seconds. Uploads and file downloads get more time in proportion to their size: 1 minute plus 1 second per 2 MiB, up to 30 minutes. A reverse proxy in front of the server needs timeouts at least this long for large uploads.

## Site isolation

- **Subdomain and custom domain modes** give each site its own browser origin. The site cannot read the dashboard's cookies or call its API as the user, and the dashboard's session cookie is `__Host-` prefixed.
- **Path mode** serves sites on the dashboard's origin, so every `/s/` response carries a `Content-Security-Policy: sandbox` without `allow-same-origin`. See [Hosting modes](hosting-modes.md#path-mode-and-its-sandbox).
- Site hosts never run API authentication, and they ignore session cookies and API keys.
- Stored files leave the API only as `application/octet-stream` with `Content-Disposition: attachment`. Only the hosting code serves them with their real type, and it never sniffs a type.
- The server rejects unsafe file paths (traversal, absolute paths, backslashes, control characters, case collisions), never creates or follows symlinks, and opens files through `os.Root` so a path cannot leave a version's directory.
- Every not-found cause (unknown host, unknown slug, disabled mode, no live version) returns the same response.

A hosted page can still run scripts against the world: it can make requests to other sites, load third-party content and show phishing pages. If your users are not trusted, host on a separate registrable domain so that a hostile site cannot damage the reputation or cookies of your dashboard's domain. See [Choosing a base domain](hosting-modes.md#choosing-a-base-domain).

## Gated sites

Password and restricted sites check a signed viewer cookie before the server looks up any file, so existing and missing paths answer alike. Every response for a gated site carries `frame-ancestors 'none'` and `X-Frame-Options: DENY`, and gated content is `Cache-Control: private`. The gate page shows that a site exists at that slug, and nothing else about it. Changing the mode, password or an allowlist moves the site's epoch and signs every viewer out. See [Access control](access-control.md).

Use subdomain or custom domain mode for gated sites. In path mode the viewer cookie must be `SameSite=None; Secure`, which means it also travels with cross-site requests to that path, and a browser that blocks third-party cookies can withhold it. Over plain HTTP, path mode cannot gate at all.

## Secrets at rest

The SMTP password, the Resend API key, the Cloudflare API token and the MaxMind licence key are write-only: the settings API never returns them. The server encrypts them with AES-256-GCM before it stores them in the database. The analytics visitor secret is sealed the same way. The server stores no viewer signing secret. It derives the keys for viewer cookies, gate form tokens and site email codes from the sealing key, so a copy of the database alone cannot forge a viewer cookie. The agent sign-in code hashes derive from the sealing key too. A changed sealing key therefore invalidates codes in flight and signs every gate viewer out.

The sealing key comes from `SECRET_KEY` when set. Otherwise the server creates `DATA_DIR/secret.key` with mode 0600 on first start.

**With SQLite, the key and the database sit together.** A backup of the whole data directory contains `selfhost.db` and `secret.key`, so anyone who gets the backup can decrypt the stored secrets. Sealing protects only against someone who reads the database without the key, such as a leaked SQL dump. To keep a data-directory backup from exposing secrets, supply `SECRET_KEY` from outside the data directory (an environment variable from your secret manager or a root-only file) and keep that value out of the backup. Then back the key up separately: without it, stored secrets cannot be read and email stays off until the root user enters them again.

With PostgreSQL the database and `DATA_DIR` are separate, so a database dump alone does not carry `secret.key` unless it is in the same backup set.

The server creates `DATA_DIR` and its subdirectories with mode 0700 and the SQLite database, its `-wal` and `-shm` files and `secret.key` with mode 0600. At every start it tightens the modes of existing ones and logs a warning if it cannot. A backup job that runs as another user needs its own access to the directory, for example through a group-readable copy that the job makes. Certificate private keys live in `TLS_STORAGE` (default `DATA_DIR/certs`) with mode 0600, unencrypted. The setup token appears once in the server log at startup, while the instance has no users. Treat the log as sensitive until setup is complete. Never log a code, token, reset link, invite link, API key or password: the server does not.

## Analytics privacy

The server stores coarse values derived from each request, a keyed daily visitor hash and, only when you configure GeoLite2, a country and city. It never stores the IP address. See [Analytics](analytics.md) for the exact list.

## Known limits

- Rate limit counters and the upload and finalize locks live in process memory. Run one server process per `DATA_DIR`. Horizontal scaling is unsupported: two processes on one data directory break the locks, and two processes with separate directories would each hold different files.
- A process restart resets rate limits.
- The server has no built-in two-factor authentication and no audit log.
- The bot filter for analytics is a short list of name fragments.
- A path-mode site runs with the dashboard's origin as its parent, protected only by the sandbox policy.
- The gate page of a gated site reveals that the slug exists.
- Built-in HTTPS accepts the certificate authority's terms of service on your behalf.
