# selfhost

A self-hosted, open source alternative to here.now: hosting for already built static sites, aimed at AI agents, with a dashboard and a REST API. It targets one person or a small team running it on a VPS. It is not a public sign-up service.

"selfhost" is a placeholder name and will change.

## Status

Early development. The server starts, loads its configuration, connects to the database, applies migrations and serves the authentication API (first-time setup, login, sessions and API keys), the workspace API (workspaces, members, folders and sites), the publishing API (versions, uploads, finalize, restore) and the instance settings API. It also serves published sites by subdomain, by path or at custom domains the owner proves they control, and gates them with per-site access modes (public, password or restricted). It counts anonymous page views without cookies or scripts (see [Analytics](#analytics)). It sends email through SMTP or Resend (see [Email](#email)) and builds on that for invites, password resets, agent sign-in and the viewer email code. It also serves a web dashboard (see [Dashboard](#dashboard)).

**A new site is public.** Anyone who can reach the server and knows or guesses its slug can read its files, until you set another access mode with `PATCH /api/v1/publish/{slug}/access` or change the default with the `sites.defaultAccessMode` setting. See [Access control](#access-control).

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

`docs/openapi.yaml` describes every endpoint (OpenAPI 3.1). Signed-in users and API keys can download it from `GET /api/v1/docs/openapi.yaml`, and an agent guide from `GET /api/v1/docs/agent-guide.md`. `GET /api/v1/instance` tells a client the public URL, the version, whether email works, which hosting modes are on, the custom domain limit per site, who may invite and where agents keep credentials. Only `GET /api/v1/setup/status`, `POST /api/v1/setup`, `POST /api/v1/auth/login`, `GET /api/v1/invites/{token}`, `POST /api/v1/invites/{token}/accept`, `POST /api/v1/auth/password-reset/request`, `POST /api/v1/auth/password-reset/confirm`, `POST /api/v1/auth/agent/request-code` and `POST /api/v1/auth/agent/verify-code` work without credentials, plus `GET /api/v1/access/handoff`, which redirects a caller without credentials to `/login`. Every other path under `/api/` returns a JSON 401, or a JSON 404 once signed in. On the management host, every other `GET` or `HEAD` path outside `/api/`, `/s/` and `/.well-known/` serves the dashboard shell, which is the one deliberate exception (see [Dashboard](#dashboard)).

- Browsers sign in with a session cookie (HttpOnly, SameSite=Lax, 30 days, renewed on use). Over HTTPS it is named `__Host-session`, is Secure, has Path=/ and no Domain attribute, so a sibling subdomain cannot overwrite it. Over plain HTTP (local use) it is named `session`. The server reads only the form that matches the request. State-changing requests that use the cookie must send an `X-Requested-With` header and an `Origin` header that matches the host.
- Scripts and agents use `Authorization: Bearer <key>` with a key from `POST /api/v1/me/keys`. The response shows the full key once. The server stores only its SHA-256 hash. Each user can hold 50 active keys.
- Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. `retry_after` appears on 429 responses.

### Rate limits

Login and setup are limited per IP address and per email: login allows 30 attempts per IP and 10 per email in 15 minutes, setup allows 10 per IP and 5 per email. The account sign-in handoff allows 30 calls per user per minute. Checking a custom domain allows 20 calls per user per minute, and claiming, changing or deleting domains allows 30 per user per minute. The counters live in process memory, so a restart resets them and each server process counts separately. A successful login resets that email's counter. The routes that work without credentials and can send email or issue credentials have their own limits per IP address and, where an email is involved, per email address: invite lookup 60 per minute and invite accept 10 per 15 minutes per IP, reset requests 5 per 15 minutes per IP and 3 per hour per email, reset confirmation 10 per 15 minutes per IP, agent code requests 10 per 15 minutes per IP, 3 per hour per email and 100 per hour for the whole instance, agent code verification 20 per 15 minutes per IP and 10 per 15 minutes per email. Creating or resending invites allows 30 per hour per user. The viewer gate has its own limits, listed under [Access control](#access-control).

### Running behind a reverse proxy

Set `TRUSTED_PROXIES` to the addresses of your proxy. The server then reads the client address from `X-Forwarded-For` and decides whether a request used HTTPS from `X-Forwarded-Proto`, but only for requests that arrive from those addresses. Without it, every client appears to be the proxy and shares one rate limit. The proxy must pass the original `Host` header, because the CSRF check compares it with `Origin` and the host router needs it to find the site.

Set `PUBLIC_URL` to the address people use for the dashboard and API, for example `https://sites.example.com`. An `https://` value makes the session cookie Secure and `__Host-` prefixed, and tells the wildcard check which scheme to probe.

For subdomain mode the proxy needs two more things. Create a wildcard DNS record, such as `*.sites.example.com`, that points to the server, and obtain a wildcard certificate for `*.sites.example.com` (DNS-01 validation) if the sites use HTTPS. Route every host under the wildcard to this server and keep the `Host` header.

To let the server handle HTTPS itself and drop the proxy, see [Built-in HTTPS](#built-in-https).

## Built-in HTTPS

The server can terminate HTTPS itself and obtain its own certificates from an ACME certificate authority, so it runs without a reverse proxy. It is off by default. Behind a proxy such as Traefik on Coolify, leave `TLS_ENABLED` unset: the proxy owns TLS and nothing in this section applies.

Built-in HTTPS changes which ports the process listens on, so you switch it on with environment variables, not with a setting. When it is on, `LISTEN_ADDR` is ignored and the server opens two listeners:

- `TLS_HTTPS_ADDR` (default `:443`) serves everything over HTTPS, with HTTP/2 and TLS 1.2 or newer.
- `TLS_HTTP_ADDR` (default `:80`) answers ACME HTTP-01 challenges and redirects every other request to the same host and path over HTTPS (301). It never reaches the API or any site.

| Variable | Default | Description |
|---|---|---|
| `TLS_ENABLED` | `false` | Switches built-in HTTPS on. `PUBLIC_URL` must then be an `https://` URL. |
| `TLS_HTTP_ADDR` | `:80` | Address of the challenge and redirect listener. |
| `TLS_HTTPS_ADDR` | `:443` | Address of the HTTPS listener. |
| `ACME_EMAIL` | none | Contact address for the ACME account. Required when `TLS_ENABLED` is true. |
| `ACME_CA` | Let's Encrypt production | URL of the ACME directory. Use `https://acme-staging-v02.api.letsencrypt.org/directory` to test without hitting the production rate limits. |
| `ACME_CA_ROOT` | unset | Path to a PEM file with the root certificate of a private CA, for an ACME server whose certificate the system does not trust, such as Pebble. |
| `TLS_STORAGE` | `DATA_DIR/certs` | Directory for the ACME account key and the certificates. The server creates it with mode 0700, and certmagic writes key files with mode 0600. Back it up with `DATA_DIR`. |

Setting `TLS_ENABLED` accepts the certificate authority's terms of service on your behalf.

Start with the staging CA, confirm that the dashboard loads over HTTPS, then remove `ACME_CA`:

```sh
docker run -d --name selfhost \
  -p 80:80 -p 443:443 \
  -v selfhost-data:/data \
  -e TLS_ENABLED=true \
  -e PUBLIC_URL=https://sites.example.com \
  -e ACME_EMAIL=alex@example.com \
  -e ACME_CA=https://acme-staging-v02.api.letsencrypt.org/directory \
  selfhost
```

The default `TLS_STORAGE` lives inside the `/data` volume, so the certificates survive a restart. Binding ports 80 and 443 needs no extra setup in Docker. When you run the binary directly as a non-root user, grant it the capability with `setcap 'cap_net_bind_service=+ep' ./selfhost` or choose higher ports and forward to them.

### Which names get a certificate

1. **The management host** (the host of `PUBLIC_URL`) gets a certificate when the server starts and renews it on its own.
2. **Verified custom domains** get a certificate the first time a visitor connects, through on-demand issuance. The server issues one only when `domains.Service.Lookup` reports the name as a verified custom domain and `hosting.customDomainsEnabled` is on. Every other name is refused before any ACME request, so a visitor cannot make the server contact the CA by sending an arbitrary host name. Refused names are never logged individually. The server logs a count of refusals at debug level at most once a minute. On-demand issuance is also limited to 5 attempts per name per hour and 50 per hour in total, which keeps the server within the CA's limits. Removing a domain or switching the feature off stops renewal.
3. **Site subdomains** use one wildcard certificate for `*.{baseDomain}`. The server never requests a certificate for a single subdomain name, because that would let any visitor trigger issuance. A subdomain name without a wildcard certificate gets a failed TLS handshake.

### Wildcard certificates for subdomain mode

A wildcard certificate needs DNS-01 validation: the server adds a TXT record at `_acme-challenge.{baseDomain}` through your DNS provider's API. Cloudflare is the first supported provider. The root user configures it with two settings:

```sh
curl -X PATCH https://sites.example.com/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"tls.dnsProvider":"cloudflare","tls.cloudflareApiToken":"<token>"}}'
```

The token is write-only and stored encrypted, like the SMTP password. The server reads it again for every DNS call, so a replaced token applies within about 20 seconds without a restart, and a failed issuance retries with the new token at once. It never appears in a log or response.

Create the token in the Cloudflare dashboard under My Profile, API Tokens, with the least privilege that works:

- Permissions: Zone, DNS, Edit and Zone, Zone, Read.
- Zone Resources: Include, Specific zone, the one zone that holds the base domain.

The server asks for the wildcard certificate as soon as the base domain is known (`hosting.baseDomain`, which defaults to the host of `PUBLIC_URL`) and the provider is configured, so the certificate can exist before you enable subdomain mode. The wildcard check probes over HTTPS and needs it. Without a provider, the server starts normally and serves subdomain sites over HTTPS only for names that already have a certificate in `TLS_STORAGE`. Root sees the wildcard state `none` in `GET /api/v1/hosting/status`, and every user sees the notice "Subdomain addresses may show certificate warnings." while subdomain mode is on and the wildcard certificate is missing or failing.

**Subdomain mode under a subdomain needs its own wildcard.** If `hosting.baseDomain` is `artifacts.example.com`, sites live at `{slug}.artifacts.example.com`, and the certificate must cover `*.artifacts.example.com`. A wildcard for `*.example.com` covers one label only, so it does not apply. Cloudflare's free universal certificate covers `example.com` and `*.example.com` only, so a proxied (orange cloud) record for `*.artifacts.example.com` shows a certificate warning. Built-in DNS-01 issues the right certificate, provided the wildcard DNS record is DNS-only (grey cloud, not proxied) so that visitors reach this server and see its certificate.

### Security headers and cookies

Over built-in HTTPS the server sends `Strict-Transport-Security: max-age=31536000` on the management host, on custom domains and on subdomain-mode sites. It sets no `includeSubDomains` and no `preload`, so each host only commits itself. It sends no HSTS over plain HTTP or when built-in HTTPS is off, because a reverse proxy may add its own. A request that arrives on the TLS listener counts as HTTPS for cookies without `TRUSTED_PROXIES`, so the session cookie is `__Host-session` and Secure.

### Port 80 and the unauthenticated surface

Built-in HTTPS adds no unauthenticated API route. The ACME challenge path `/.well-known/acme-challenge/{token}` is answered by the certificate library on the port-80 listener before the redirect, and only for tokens the server itself issued. The API router never sees port 80. The TLS-ALPN challenge is answered inside the TLS handshake on `TLS_HTTPS_ADDR`.

### Status

Root sees `detail.tls` in `GET /api/v1/hosting/status`:

- `enabled` is true when the server runs with `TLS_ENABLED`.
- `managementCertificateExpiresAt` is the expiry of the management host certificate.
- `wildcard.state` is `none`, `pending`, `ok` or `failed`, with a short `reason` for `none` and `failed`. The full error is in the server log.
- `customDomains.count` and `customDomains.soonestExpiresAt` cover verified custom domains with a loaded certificate.
- `dnsProviderConfigured` is true when a provider and a token are stored.

### Testing against a local ACME server

The tests in `internal/certs` run certmagic against Pebble when `PEBBLE_DIRECTORY`, `PEBBLE_CA_ROOT` and `PEBBLE_DNS` are set, and skip otherwise. Start Pebble with `docker run --rm --network host -e PEBBLE_VA_NOSLEEP=1 ghcr.io/letsencrypt/pebble:latest -config /test/config/pebble-config.json -dnsserver 127.0.0.1:8054`, copy `/test/certs/pebble.minica.pem` out of the image, and point `PEBBLE_CA_ROOT` at it. `PEBBLE_DIRECTORY` is `https://localhost:14000/dir` and `PEBBLE_DNS` is `127.0.0.1:8054`. The tests use ports 5001 and 5002.

## Dashboard

The dashboard is a web app for the people who use the server. Every feature the API offers to a signed-in user has a screen, except publishing: it has no way to upload or publish a site, because agents do that through the API.

- **Sites.** The home page lists the sites you can reach across your workspaces, with their addresses, access modes and availability notices. It manages folders, moves sites between them and searches by name and address. A site shows a warning in the list only when it cannot be reached or when the system switched an address mode off after failed checks. A reachable site whose other address mode is simply off shows no warning, and the site page explains each notice in quiet text.
- **Site pages** (`/sites/{slug}`). Overview (name, description, single-page app mode, page view counting, folder, delete with a typed confirmation), Access (public, password or restricted, the password field, email and domain allowlists), Domains (custom domains with the DNS challenge, check now, primary and delete), Analytics (range presets, a custom range, views and unique visitors as an accessible SVG chart with a table alternative, ranked lists and bot views apart) and Versions (history, restore, delete and a read-only list of the live files).
- **Workspaces** (`/workspaces/{slug}`). The sites view plus Members (grants for the whole workspace, one folder or one site, role changes, removal), Invites (create, resend and revoke, with the link shown once), Analytics (a workspace-wide role is needed) and Settings (rename, delete when empty).
- **Settings** (`/settings`). Profile, appearance and server facts, API keys (the full key is shown once), personal workspace analytics and an overview of every custom domain on your sites. The API has no call to change a display name, an email address or a password while signed in, so the dashboard offers none. A forgotten password goes through a reset link.
- **Instance admin** (`/admin`, root only, a 404 for everyone else). Hosting (addresses, wildcard check, custom domains, certificate state, DNS provider), Email (provider, write-only secrets, a test message), Users (disable, enable, reset links, invites), Access and policy (who invites, who creates workspaces, default access mode, retention and upload limits) and Analytics (defaults, retention, MaxMind, the GeoLite2 status and an instance-wide report). Settings forms send only the fields you changed, show the server's validation message beside the field, and ask before a change that breaks something. Secrets are write-only: the dashboard shows whether one is set and never its value.

Invite links, password reset links and new API keys are shown once, in a dialog, and are gone when you close it. The first-run setup, sign-in, invite and password reset screens are part of the dashboard.

It lives in `web/` (Vite, React, TypeScript, Tailwind CSS, shadcn/ui components, react-router and TanStack Query) and uses the same REST API as agents. Build it before you build the server, because the server embeds the output:

```sh
pnpm --dir web install
pnpm --dir web build
go build ./cmd/selfhost
```

The build writes to `internal/ui/bundle`, which the Go build embeds with `go:embed`. The built files are not committed. A fresh clone still compiles, and the management host then answers its dashboard routes with a plain "dashboard not built" text and status 503. The Docker image builds the dashboard in its own stage. For development, run the server and then `pnpm --dir web dev`, which proxies `/api` to `127.0.0.1:8080`.

The management host serves the shell and its hashed assets to anyone, so that the sign-in screens can load. They hold nothing about your instance. Hashed files under `/assets/` are cached for a year, and the shell is revalidated on every load. Every response carries `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY` and a Content-Security-Policy that allows only the server's own scripts, styles, fonts and requests. Site hosts never serve the shell.

`web/src/api/schema.d.ts` is generated from `docs/openapi.yaml`. Run `pnpm --dir web gen:api` after changing the spec. The checks are `pnpm --dir web lint`, `typecheck`, `test` and `build`. `pnpm --dir web e2e` builds the server and, for each of desktop (1280 px) and phone (390 px) widths in light and dark, starts it on a temporary data directory and drives it in Chrome through setup, publishing, every site, workspace, settings and admin screen, a second user who accepts an invite in a fresh browser context, and checks for console errors and horizontal scroll. The strict Content-Security-Policy rules out inline styles and scripts, so the dashboard uses native `<dialog>` elements and draws charts as plain SVG, with no chart library. It needs `playwright-core` on `NODE_PATH` and Chrome at `CHROME_PATH` (default `/usr/bin/google-chrome`), and writes screenshots to `web/e2e/output`.

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

## Publishing

Publishing takes three steps, and a version serves nothing until the last one succeeds.

1. `POST /api/v1/publish` creates a site and a pending version. `PUT /api/v1/publish/{slug}` creates a pending version of an existing site. The body lists every file of the version with its `path`, `size`, `sha256` and `contentType`. The response holds one upload URL per file.
2. `PUT` each file's raw bytes to its upload URL. The server streams the body to a temporary file under `DATA_DIR/tmp`, checks the size and SHA-256 against the manifest, and moves the file to `DATA_DIR/sites/<siteId>/versions/<versionId>/files/<path>`. A mismatch stores nothing.
3. `POST /api/v1/publish/{slug}/versions/{versionId}/finalize` checks that every file exists with the right size and hash, checks the limits, and flips the site's live pointer in one transaction. A missing file returns 409 `missing_files` with the paths to upload again. Calling finalize again returns the same result.

Pass `baseVersionId` on `PUT /publish/{slug}` to detect conflicts. If the site's live version is not that id, the server returns 409 `version_conflict` with `currentVersionId`, and `GET /api/v1/sites/{slug}/files` lists the live files so an agent can reconcile. Finalize checks the base again inside its transaction, so two publishes built on the same base cannot both go live. Without `baseVersionId`, a publish overwrites whatever is live.

`GET /api/v1/publish/{slug}/versions` lists versions. `POST .../versions/{versionId}/restore` makes an older finalized version live without uploading, and `DELETE .../versions/{versionId}` removes one unless it is live. After each finalize the server deletes finalized versions beyond `retention.keepVersions`, never the live one. Pending versions that are not finalized within 24 hours expire, and an hourly maintenance task removes them, orphaned directories and stale upload files. Deleting a site removes its directory.

`GET /api/v1/sites/{slug}/files/{path}` returns a live file as `application/octet-stream` with `Content-Disposition: attachment`, so a stored HTML file never runs on the API origin.

Paths are relative with `/` separators. The server rejects empty and absolute paths, `.` and `..` segments, empty segments, backslashes, NUL, control and invisible formatting characters, paths over 1024 bytes and segments over 255 bytes. It also rejects two paths that differ only in letter case, and a path that is also the directory of another. Dotfiles such as `.well-known/` are allowed. The server never creates or follows symlinks.

Set a site's `spaMode` when creating it or with `PATCH /api/v1/sites/{slug}`. In SPA mode, a request that matches no file and whose last path segment has no file extension serves `index.html` with status 200.

Several processes must not share one `DATA_DIR`. The upload and finalize locks live in process memory.

## Hosting sites

The server serves each site's live version in two ways. A public site needs no sign-in, and the other [access modes](#access-control) put a gate in front of the files.

| Mode | Address | Setting |
|---|---|---|
| Subdomain | `https://{slug}.{baseDomain}/` | `hosting.subdomainEnabled`, off until a wildcard check passes |
| Path | `{PUBLIC_URL}/s/{slug}/` | `hosting.pathEnabled`, on by default |
| Custom domain | `https://{your domain}/` | `hosting.customDomainsEnabled`, on by default. See [Custom domains](#custom-domains) |

Every site response and every publish response carries a `url` field with the address the instance serves the site at right now: the primary custom domain when the site has one, else the subdomain address when subdomain mode is on, else the path address when path mode is on, else the first verified custom domain by name, else `null`.

**Use subdomain mode.** Each site then gets its own browser origin, so its JavaScript can use cookies and `localStorage` and cannot touch the dashboard. Path mode puts sites on the dashboard's origin. To stop hosted JavaScript from acting as a signed-in user, the server sends `Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups allow-downloads allow-modals allow-presentation` on every `/s/` response. The policy omits `allow-same-origin`, so a path-mode page runs in an opaque origin. Its scripts run, but it cannot read cookies or `localStorage`, and it cannot call the API as the signed-in user. Path-mode responses also carry `Access-Control-Allow-Origin: *`, so module scripts and fonts load from the opaque origin. Subdomain-mode sites get no policy, because their owners control their own content. A path-mode site lives under `/s/{slug}/`, so its pages must use relative links. A link such as `/style.css` points at the dashboard host, not at the site.

### Routing

The server normalises the request host (it drops the port and a trailing dot and lower-cases it) and then decides:

- The host of `PUBLIC_URL` is the management host. It serves the API, and `/s/{slug}/...` serves a site in path mode. `/s/{slug}` redirects to `/s/{slug}/`. A site's files are never reachable on the management host except through `/s/`.
- `{slug}.{baseDomain}` with subdomain mode on serves that site. The slug must be a single DNS label.
- A host that is a verified custom domain, while `hosting.customDomainsEnabled` is on, serves the site that owns it exactly as subdomain mode does. The router checks the management host first, then the subdomain pattern, then the custom domains.
- Every other host serves nothing from the API. Site hosts never run API authentication: they ignore session cookies and API keys. A gated site has its own viewer cookie, described under [Access control](#access-control).

An unknown host, an unverified custom domain, a custom domain while the feature is off, an unknown slug, a disabled mode and a site without a live version all return the same 404 response with the same headers and a one-line body, so a visitor cannot tell them apart. Only `GET` and `HEAD` work on site hosts and under `/s/`, apart from the `POST` forms of the gate on gated sites. Other methods return 405 with `Allow: GET, HEAD`.

`hosting.baseDomain` defaults to the host name of `PUBLIC_URL`. Changing it to a different value resets the wildcard check and switches subdomain mode off. If that would leave both modes off, the server switches path mode on in the same change and the response carries a notice with the code `path_mode_enabled`.

At least one mode stays on. A settings change that would leave `hosting.subdomainEnabled` and `hosting.pathEnabled` both false returns 409 `no_hosting_mode`. The server judges the stored values merged with the patch, so one patch can switch one mode on and the other off.

### What users see

Every site response and publish response has a `hosting` object:

- `reachable` is true when the site has a live version and at least one mode can serve it now. A site with a password or restricted access does not count path mode over plain HTTP, because the gate needs a secure connection there. A verified custom domain counts while custom domains are enabled, even when subdomain and path mode are off for the site.
- `url` is the same address as the top-level `url`.
- `modes.subdomain`, `modes.path` and `modes.customDomains` each have a `state` (`on` or `off`) and a `reason`. The reason is `switched_off`, `wildcard_check_failed`, `https_required` or `not_applicable`, and null when the mode is on and healthy.
- `notices` lists `{code, message}` entries in plain language, such as "The subdomain address for this site is switched off.", "Nothing has been published to this site yet." and "A custom domain is waiting for DNS verification.". They carry no administrator detail.

Every user with read access to a site gets the same `hosting` object, root or not. A user sees nothing about sites they cannot read.

`GET /api/v1/hosting/status` gives any signed-in user the instance view: the state and reason of each mode, notices, and `degraded`, which is true while subdomain mode is on and its last wildcard check failed. The root user also gets a `detail` object with the check status, when it last ran, the failure text, the consecutive failure count, the reason subdomain mode was switched off and a `tls` object with the certificate state (see [Built-in HTTPS](#built-in-https)). Other users never get `detail`.

### Which file a path serves

For a request path `/p`, the server tries `p`, then `p.html`, then `p/index.html`. `/` serves `index.html`. A path with a trailing slash (`/p/`) tries `p/index.html`, then `p.html`. When only `p/index.html` exists, `/p` redirects once to `/p/`, which keeps relative links working. Next.js static exports (`about.html`, `docs/index.html`, `_next/...`) work as built. When nothing matches, the server serves the site's `404.html` with status 404 if it exists, and the plain 404 otherwise. SPA mode comes before `404.html` for paths without a file extension.

Responses support `Range`, `If-None-Match` and `If-Modified-Since`. The `ETag` is the stored SHA-256. The content type is the one stored at publish time (guessed from the extension when the manifest left it empty), and the server never sniffs it. `X-Content-Type-Options: nosniff` is always set. HTML responses send `Cache-Control: no-cache` and every other file sends `Cache-Control: public, max-age=3600, must-revalidate`. A new version is live as soon as finalize or restore returns. The server never serves a symbolic link or anything outside the version's directory.

### Wildcard check

Subdomain mode needs wildcard DNS, which the server cannot see from inside. The check proves it: the server picks a random label, resolves `{label}.{baseDomain}`, then requests `{PUBLIC_URL scheme}://{label}.{baseDomain}/.well-known/selfhost-check/{token}` and passes only when the server itself answers. The answer is an HMAC of the one-time token under a secret that never leaves the process, so another server that receives the request cannot fake it. It does not follow redirects, reads at most 256 bytes and gives up after 10 seconds. A pass proves DNS, the wildcard certificate (for HTTPS) and the proxy routing in one step.

The token route is the one unauthenticated path this feature adds. It exists only on site hosts, only while a check is running, accepts each token once for 60 seconds and returns the plain 404 otherwise.

The server runs the check shortly after startup (with up to three attempts) and every 6 hours. The root user can run it at any time with `POST /api/v1/admin/hosting/check`, limited to 6 calls per minute. The result lands in the read-only settings `hosting.wildcardStatus` (`unchecked`, `ok` or `failed`), `hosting.wildcardCheckedAt` and `hosting.wildcardDetail`. Setting `hosting.subdomainEnabled` to true while the status is not `ok` returns 409 `wildcard_not_verified`.

A failed check while subdomain mode is on does not stop it. The server keeps serving, records status `failed` with the detail, and adds 1 to `hosting.wildcardFailures`. It then retries every 15 minutes until a check passes. After 3 consecutive failed startup or scheduled checks it switches `hosting.subdomainEnabled` off and sets `hosting.subdomainDisabledReason` to `wildcard_check_failed`, but only when path mode is on. It never leaves both modes off. A passing check resets the counter and clears the reason. It does not switch subdomain mode back on, so enable it again after fixing DNS. A check run through `POST /api/v1/admin/hosting/check` records its result but does not count toward the 3.

On a local setup with `PUBLIC_URL=http://localhost:8080`, the check fails unless your resolver maps `*.localhost` to the loopback address. Path mode still works.

## Custom domains

A site can have several custom domains, such as `docs.example.org`. Each domain belongs to one site on the whole instance. A verified domain serves the site exactly as its subdomain does: no sandbox policy, host-only cookies, and the same access modes and gate. No domain redirects to another. One verified domain can be the site's primary domain, which is the address the site reports in `url`.

### Adding a domain

1. `POST /api/v1/sites/{slug}/domains` with `{"domain": "docs.example.org"}`. The server lower-cases the name, removes a trailing dot and converts international names to punycode. It refuses IP addresses, wildcards, ports, `localhost`, reserved top-level domains (`.local`, `.internal`, `.test` and similar), single-label names, the host of `PUBLIC_URL`, the base domain and every name under it.
2. The response is a `pending_verification` domain with a `challenge`: a TXT record to create at `_selfhost-challenge.docs.example.org`, whose value is a random 192-bit token. It also has a `target` hint: a CNAME to the host of `PUBLIC_URL`.
3. Create the TXT record at your DNS provider, then call `POST /api/v1/sites/{slug}/domains/{id}/verify`. The server resolves the TXT records and marks the domain verified when one matches the token. The response says in plain language why a check failed (no record yet, a record with another value, or a lookup that failed or timed out). DNS changes can take a while to spread, so repeat the call. It is limited to 20 per minute per user.
4. Optionally make it the primary domain with `PATCH /api/v1/sites/{slug}/domains/{id}` and `{"isPrimary": true}`. Setting a new primary clears the old one in the same step.

Viewing domains needs read access to the site. Adding, verifying, changing and deleting need write access. `DELETE /api/v1/sites/{slug}/domains/{id}` stops serving at once and frees the name.

### Why a TXT record

The server proves control of the name, not of the address it points to. A domain behind a CDN or proxy resolves to the proxy, not to this server, so an A or CNAME check would fail for exactly the setups that need it. A TXT record at a name only the domain owner can edit works everywhere. Verification never looks at A, AAAA or CNAME records.

### Pointing the domain at the server

A visitor must still reach this server. Use a CNAME to the host of `PUBLIC_URL` for a subdomain such as `docs.example.org`. A domain with no subdomain part (an apex domain such as `example.org`) usually cannot hold a CNAME, so use A and AAAA records with the same addresses as the `PUBLIC_URL` host, or your DNS provider's alias or flattening feature. A CDN or proxy in front can sit between the visitor and this server if it forwards the original `Host` header.

### Certificates for custom domains

With built-in HTTPS on, the server requests a certificate for each verified custom domain the first time a visitor connects. See [Built-in HTTPS](#built-in-https).

Behind a reverse proxy, the proxy owns TLS. It must route each custom domain's host to this server with the `Host` header intact, and it must hold a certificate for each one. A wildcard certificate for the base domain does not cover a custom domain. With Traefik (including the one Coolify manages) or Caddy, add the custom domain as a host that the proxy routes to this server. Whether the proxy requests a certificate for it on its own depends on how you configured it, so check your proxy's documentation.

### Ownership rules

- A claim blocks every other site from the same name while it is pending. A pending claim that has not been verified after 7 days is released by the maintenance task. A verified domain is released only by deleting it.
- An unverified domain never serves. A request for it gets the same 404 as an unknown host.
- The maintenance task re-checks every verified domain once a day. A lookup that succeeds without the token, or finds no record, counts as a miss. After 3 consecutive daily misses the domain goes back to `pending_verification`, stops serving and stops being primary. The site and its other domains are untouched. A timeout or resolver error is not a miss, so a DNS outage never takes domains down. The domain keeps `lastCheckedAt` and `lastCheckResult` for the last lookup, and the 7-day clock for pending claims restarts when a domain drops.
- Verified domains work for gated sites. The gate page on a custom domain sends the sign-in link with a `host` parameter, and `GET /api/v1/access/handoff` returns to that domain only if it is a verified domain of the same site. The viewer cookie is host-only and named like the subdomain-mode cookie (`__Host-viewer` when the request arrived over HTTPS, `viewer` otherwise).

`hosting.customDomainsEnabled` (default true) turns serving off for every custom domain at once without touching stored domains. `hosting.maxDomainsPerSite` (default 10, 1 to 50) caps the domains one site holds. Changing `hosting.baseDomain` to a name that equals or contains a custom domain returns 409 `base_domain_in_use`.

## Access control

Every site has one access mode. New sites start in the mode of the `sites.defaultAccessMode` setting, which is `public` unless you change it.

| Mode | Who gets in |
|---|---|
| `public` | Anyone with the link. |
| `password` | Anyone who knows the site's password. |
| `restricted` | Instance users who have at least read access to the site (through a workspace role or a grant scoped to the site or its folder), and viewers whose email address or email domain is on the site's allowlist. An empty allowlist means members only. |

Read and change the settings with `GET` and `PATCH /api/v1/publish/{slug}/access`. Reading needs read access to the site and changing needs write. `mode`, `password` (a string, or `null` to remove it), `emailAllowlist` and `domainAllowlist` can be sent together or alone. The lists replace the stored lists, each holds at most 500 entries, and a domain entry such as `example.com` matches addresses at exactly that domain. Password mode needs a password to exist. Passwords have 8 to 128 characters, are stored as argon2id hashes and are never returned or logged. Responses show `hasPassword` instead. `accessMode` also appears on every site.

Changing the mode, the password or either list moves the site's access epoch. Every viewer cookie carries the epoch it was issued under, so one change signs every viewer out. It also discards unused email codes and sign-in links for the site. Removing a user's workspace access takes effect at once too, because the server checks a member's access again on every request.

### What a visitor sees

A request for a gated site gets the gate page, an HTML page with status 401 and no site content. Requests that are clearly not page loads (scripts, images and `fetch` calls) get a plain 401. The gate runs before the server looks up any file, `404.html` included, so a path that exists and one that does not get the same answer. Gate pages are server-rendered with inline CSS, no JavaScript and no external requests.

- A **password** site shows a password form. A wrong password gets the same message whatever the reason. Unlock attempts are limited to 10 per IP and 100 per site in 15 minutes, and a limited request gets 429 with `Retry-After` (or a JSON `retry_after` for `Accept: application/json`).
- A **restricted** site shows a link that signs in an instance user, and, when the instance can send email, a form for allowlisted viewers. A viewer enters an email address and receives an 8-digit code that works once, for 10 minutes and for 5 attempts. The page answers the same way whether or not the address is on the allowlist, and only allowlisted addresses receive mail. Code requests are limited per address (3 per 10 minutes), per IP (10 per 15 minutes) and per site (60 per hour). Code checks are limited per address, per IP and per site as well.
- **Email codes need an email sender.** SMTP and Resend senders arrive in a later step. Until then the instance reports `emailCodeAvailable: false`, the gate page says email sign-in is not available, and allowlisted addresses are stored but cannot sign in. Instance users can still use the account link.

The account link goes to `GET /api/v1/access/handoff?site=<slug>&return=<path>` on the management host. On a custom domain it also carries `host=<domain>`, and the redirect goes back to that domain. A caller with a session or API key and read access gets a single-use token, valid for 60 seconds and for that site only, and a redirect to the site host, which exchanges it for the viewer cookie and redirects to `return`. A caller without credentials is redirected to `/login?next=...`. The sign-in page is part of the dashboard. The return path must be a path on the site: absolute URLs, protocol-relative URLs, backslashes, control characters, fragments and dot segments are refused, including percent-encoded forms.

### Viewer cookies

A successful sign-in sets a signed cookie (HMAC-SHA256 over the site id, the epoch, the expiry and the method) that lasts 7 days. The signing key is a random instance secret, created on first use and kept in the `instance_settings` table under a key the settings API neither returns nor accepts. A cookie from one site never opens another.

| Mode | Name | Attributes |
|---|---|---|
| Subdomain, HTTPS | `__Host-viewer` | `Path=/`, `Secure`, `HttpOnly`, `SameSite=Lax`, no `Domain` |
| Subdomain, plain HTTP | `viewer` | `Path=/`, `HttpOnly`, `SameSite=Lax` |
| Custom domain, HTTPS | `__Host-viewer` | Same as subdomain mode over HTTPS |
| Custom domain, plain HTTP | `viewer` | Same as subdomain mode over plain HTTP |
| Path, HTTPS | `__Secure-viewer` | `Path=/s/<slug>/`, `Secure`, `HttpOnly`, `SameSite=None` |

**Path mode behaves differently.** The sandbox gives a path-mode page an opaque origin, so the browser treats its requests for scripts, styles and images as cross-site and does not send `SameSite=Lax` cookies. The path-mode cookie therefore uses `SameSite=None; Secure`, scoped to the site's path. **Gated sites cannot work in path mode over plain HTTP**, because browsers refuse `Secure` cookies there. The server shows a page that says HTTPS is required instead of failing silently. Even over HTTPS, a browser that blocks third-party cookies may withhold the cookie from a path-mode page's own subresources. Use subdomain mode for gated sites.

Every response for a gated site, gate pages included, carries `Content-Security-Policy: frame-ancestors 'none'` and `X-Frame-Options: DENY`, so other pages cannot frame it. Public sites stay embeddable. Gated content is served with `Cache-Control: private` and gate pages with `private, no-store`. The gate forms carry a token tied to the site, its epoch and the visitor's IP address, and the server rejects a form posted from another origin.

### Routes the gate adds

On a site host, and under `/s/<slug>/` in path mode, a gated site answers four paths that need no credentials: `GET /.well-known/selfhost-auth` (exchanges a sign-in token) and `POST /.well-known/selfhost-gate/unlock`, `/code` and `/verify` (the gate forms). They exist only while the site is gated. On a public site they behave like any other path, and a site's own file at that path is shadowed only while the site is gated. The gate counters live in process memory, like the other rate limits.

### Trade-off: the gate page shows that a site exists

A gated site that has a live version answers with its gate page, so anyone who knows or guesses the slug can tell it exists. An unknown slug, and a site without a live version, still get the plain 404. The gate page shows nothing else about the site: no name, no description and no file names. If the existence of a slug is itself sensitive, choose slugs that cannot be guessed (the generated ones are).

## Analytics

The server counts page views itself, from the requests it serves. It injects no script into your sites, sets no cookie and needs no consent banner.

### What is recorded

A view is an HTML document served to a `GET` request with status 200, or a custom `404.html` served with status 404. For each view the server stores:

- the site, the time and the request path without its query string or fragment (at most 200 characters; for a single-page app it is the path the visitor asked for);
- the status class (`2xx` or `4xx`);
- the host of the referring page (`news.example.org`, not the path or query), left empty when the visitor came from the same host or sent none;
- country and city, when a GeoLite2 database is installed;
- browser family, operating system family and device type (`desktop`, `mobile`, `tablet`, `bot` or `other`), parsed from the `User-Agent` header;
- a visitor hash.

The server never stores the IP address, the raw `User-Agent`, a query string, a cookie or any other header value. It uses the IP address in memory to look up the location and to compute the visitor hash, then drops it. The address comes from `internal/proxy`, so a forwarded address counts only when the connection comes from a `TRUSTED_PROXIES` address.

The server does not count asset requests (scripts, styles, images, data), `HEAD` requests, partial (`Range`) and cached (304) responses, prefetches, redirects, gate pages and sign-in prompts, or any request while analytics is off for the site. A protected site records a view only after the gate has let the request through.

### How unique visitors work

The visitor hash is an HMAC-SHA256 over the client address (the first 64 bits for IPv6), the `User-Agent`, the site id and the UTC date. The key is a random secret created on first use and stored encrypted in the database, and no API returns it. The same browser on the same site on the same day gets the same hash, so it counts once. The next day it gets a different hash, and so does another site. The hash cannot be reversed or linked across days. This has two consequences you should know about:

- Unique counts for a range add up the daily counts, so a person who visits on three days counts three times.
- Two people behind one address with the same browser string count as one.

Visitors whose `User-Agent` looks like a crawler or an HTTP client, or is empty, count as `botViews` and as the `bot` device. They are left out of every other number. The bot list is a short built-in list of name fragments, so it catches common bots and not all of them.

### Privacy in plain words

Visitors are not tracked across sites or days, and nothing on their device is read or written. The server keeps counts of anonymous page views. If you also turn on location lookups, it keeps country and city for each view, which are derived from the IP address but do not identify a person. Whether that fits your legal duties is your decision.

### Turning it on and off, and reading it

Each site has an `analyticsEnabled` switch (`PATCH /api/v1/sites/{slug}`). New sites take the `analytics.enabledByDefault` value. Switching a site off stops counting at once and keeps the data already recorded.

| Endpoint | Who may read it |
|---|---|
| `GET /api/v1/sites/{slug}/analytics` | Anyone with read access to the site. |
| `GET /api/v1/workspaces/{workspace}/analytics` | Users with a workspace-wide role. The report adds up the sites in the workspace that the user can see. It works for personal workspaces too. |
| `GET /api/v1/admin/analytics` | The root user. It covers every site and adds `droppedEvents`. |

Query parameters are `from` and `to` (RFC 3339 dates, both days inclusive, default the last 30 days, at most 366 days), `interval` (`day`, or `hour` for ranges of 14 days or less) and `top` (entries per ranked list, 1 to 50, default 10). The response holds totals, a time series, ranked lists for paths, referrers, countries, cities, browsers, operating systems, device types and status classes, `locationAvailable` and `generatedAt`.

### Performance

Serving never waits for analytics. The server puts each view in a bounded in-memory buffer (4096 events) and a background worker writes batches every 5 seconds or 200 events. When the buffer is full the server drops the event and adds one to `droppedEvents`, which the root report shows. The count restarts at 0 when the server starts. A failed database write drops only the affected events. A graceful shutdown writes what is still in the buffer. A crash loses at most a few seconds of events.

### Rollups and retention

Individual events stay for `analytics.rawDays` days (default 31). A maintenance task runs every hour and folds each finished UTC day into daily rollups per site (views and uniques in total and by path, referrer host, country, city, browser, operating system, device and status class, keeping the top 500 values of each list per day). Reports read the rollup for days that have one and raw events for the rest, so a range that spans both counts every view once. Running the task again changes nothing, and events that arrive late for a finished day are folded in at the next run. The server deletes raw events older than `analytics.rawDays` once they are rolled up, and deletes everything older than `analytics.retentionDays` (default 395 days, 0 keeps all). Hourly charts need raw events, so `analytics.rawDays` has a minimum of 14.

### Country and city lookups with MaxMind

Location lookups use the free GeoLite2 City database from MaxMind. To switch them on:

1. Create a free GeoLite2 account at MaxMind and generate a licence key in the account portal.
2. Set `analytics.maxmindAccountId` (the numeric account ID) and `analytics.maxmindLicenseKey` with `PATCH /api/v1/admin/settings`.

Within five minutes the server downloads `GeoLite2-City` from `download.maxmind.com` over HTTPS, checks its SHA-256 checksum when MaxMind provides one, and installs it as `DATA_DIR/geo/GeoLite2-City.mmdb` through an atomic rename, so a failed or partial download never replaces a working file. It refreshes the database at most once a week, and retries a failed download after 6 hours or at once when you change the credentials. The lookup picks up a new file without a restart. `analytics.geoStatus`, `analytics.geoUpdatedAt`, `analytics.geoCheckedAt` and `analytics.geoDetail` show the state to the root user, and the licence key is never logged or returned. Without credentials the location fields stay empty and reports show `locationAvailable: false`.

The GeoLite2 database and the account are MaxMind's, and MaxMind's GeoLite End User License Agreement governs their use, including the duty to keep the data current and its attribution requirements. Read it before you switch lookups on. MaxMind limits GeoLite2 downloads per day.

## Instance settings

The root user reads and changes instance settings with `GET` and `PATCH /api/v1/admin/settings`. Other users get 404. A `PATCH` sends only the keys to change, and the server rejects unknown keys and invalid values without applying any.

| Key | Default | Meaning |
|---|---|---|
| `retention.keepVersions` | 10 | Finalized versions kept per site, counting the live one. Minimum 1. |
| `limits.maxFilesPerVersion` | 2500 | Files in one version. |
| `limits.maxVersionBytes` | 1073741824 | Total bytes in one version. |
| `limits.maxFileBytes` | 268435456 | Bytes in one file. Also caps the upload body. |
| `hosting.baseDomain` | host of `PUBLIC_URL` | Bare host name for site subdomains, such as `example.com`. An empty value means the default. |
| `hosting.subdomainEnabled` | false | Serve sites at `{slug}.{baseDomain}`. Needs a successful wildcard check. At least one of the two modes must stay on. Subdomain mode is the recommended way to host sites. |
| `hosting.pathEnabled` | true | Serve sites at `/s/{slug}/`. Path-mode pages run in a sandbox without cookies or `localStorage`. |
| `hosting.customDomainsEnabled` | true | Serve verified custom domains. Switching it off makes every custom host answer with the unknown-host 404 and leaves stored domains alone. |
| `hosting.maxDomainsPerSite` | 10 | Custom domains one site can hold, from 1 to 50. |
| `hosting.wildcardStatus` | `unchecked` | Read-only. Outcome of the last wildcard check: `unchecked`, `ok` or `failed`. |
| `hosting.wildcardCheckedAt` | null | Read-only. When the last check finished. |
| `hosting.wildcardDetail` | empty | Read-only. Why the last check failed. |
| `hosting.wildcardFailures` | 0 | Read-only. Consecutive failed startup or scheduled checks while subdomain mode was on. |
| `hosting.subdomainDisabledReason` | null | Read-only. `wildcard_check_failed` when the server switched subdomain mode off after 3 failed checks. |
| `sites.defaultAccessMode` | `public` | Access mode of sites created from now on: `public` or `restricted`. Existing sites keep theirs. `password` is not allowed, because a password is set per site. |
| `invites.whoCanInvite` | `root` | Who may create invites: `root` or `workspaceAdmins`. With `workspaceAdmins`, a workspace admin may invite only into a workspace they administer. |
| `workspaces.whoCanCreate` | `any` | Who may create shared workspaces: `any` user or only `root`. |
| `email.provider` | `none` | `none`, `smtp` or `resend`. |
| `email.fromAddress` | empty | Sender address, such as `noreply@example.com`. Both providers need it. |
| `email.fromName` | empty | Optional sender name. |
| `email.smtp.host` | empty | SMTP server host name or IP address. |
| `email.smtp.port` | 587 | SMTP server port. |
| `email.smtp.security` | `starttls` | `tls` (implicit TLS, usually port 465), `starttls` (usually port 587) or `none`. |
| `email.smtp.username` | empty | SMTP user name. Leave empty for a server without authentication. |
| `email.smtp.password` | write-only | SMTP password. Reads return `email.smtp.passwordSet` (true or false) instead. An empty string clears it. |
| `email.resend.apiKey` | write-only | Resend API key. Reads return `email.resend.apiKeySet` instead. An empty string clears it. |
| `tls.dnsProvider` | `none` | DNS provider for the wildcard certificate: `none` or `cloudflare`. Only used with built-in HTTPS. |
| `tls.cloudflareApiToken` | write-only | Cloudflare API token with Zone:DNS:Edit and Zone:Zone:Read on the base domain's zone. Reads return `tls.cloudflareApiTokenSet` instead. An empty string clears it. |
| `analytics.enabledByDefault` | true | Whether sites created from now on count page views. Existing sites keep their own setting. |
| `analytics.retentionDays` | 395 | Days of rolled-up analytics to keep. 0 keeps everything, otherwise 30 to 3650. |
| `analytics.rawDays` | 31 | Days of individual events to keep before only the daily rollups remain. 14 to 365. |
| `analytics.maxmindAccountId` | empty | Account ID of a free MaxMind account (digits only). Needed with the licence key for country and city lookups. |
| `analytics.maxmindLicenseKey` | write-only | MaxMind licence key. Reads return `analytics.maxmindLicenseKeySet` instead. An empty string clears it. |
| `analytics.geoStatus` | `unconfigured` | Read-only. Outcome of the last GeoLite2 download: `unconfigured`, `ok` or `failed`. |
| `analytics.geoUpdatedAt` | null | Read-only. When a GeoLite2 database was last installed. |
| `analytics.geoCheckedAt` | null | Read-only. When the last download attempt ran. |
| `analytics.geoDetail` | empty | Read-only. Why the last attempt failed, or a note about the last download. |

Changes take effect on the next request. Email settings need no restart.

## Email

Email is optional. Without it, the server still works, and each feature that needs email says so: the invite response sets `emailAvailable` to false and still returns the invite link, password reset requests send nothing, the viewer email code stays off and agent email sign-in returns `email_not_configured`.

The root user sets it up with `PATCH /api/v1/admin/settings`.

For SMTP:

```sh
curl -X PATCH http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"email.provider":"smtp","email.fromAddress":"noreply@example.com","email.fromName":"Acme Ltd",
       "email.smtp.host":"smtp.example.com","email.smtp.port":587,"email.smtp.security":"starttls",
       "email.smtp.username":"alex","email.smtp.password":"change-me"}}'
```

For Resend:

```sh
curl -X PATCH http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"email.provider":"resend","email.fromAddress":"noreply@example.com","email.resend.apiKey":"re_example"}}'
```

Email counts as configured only when the provider is set and its required fields are present: SMTP needs a host, a valid port, a security mode and the from address (and a password when it has a user name), Resend needs an API key and the from address. `POST /api/v1/admin/email/test` (root only, at most 5 per minute) sends a test message to the root user's address and returns either `{"sent":true}` or a short reason such as "the server rejected the username or password". The reason never contains a secret or the server's raw reply.

Messages are plain text. Links in them start with `PUBLIC_URL`.

### Secrets at rest

The SMTP password, the Resend API key, the Cloudflare API token and the MaxMind licence key are write-only. The settings API never returns them, only whether one is set. The server encrypts them with AES-256-GCM before it stores them. The key comes from `SECRET_KEY` (32 random bytes, base64 encoded) when set. Otherwise the server creates `secret.key` in `DATA_DIR` with mode 0600 on first start and reuses it. Generate a key with `openssl rand -base64 32`. Back up the key with the database: without it, stored secrets cannot be read, and email stays off until the root user enters them again. The analytics visitor secret is stored the same way. If it cannot be opened, the server creates a new one, which only means that visitors counted before the change can count once more on the same day. The server derives the agent sign-in code hashes from the same key, so a changed key also invalidates codes that are in flight.

## Invites

An invite creates an instance account. Nobody can sign up on their own. By default only the root user can invite. Set `invites.whoCanInvite` to `workspaceAdmins` to let a workspace admin invite into a workspace they administer. Creating shared workspaces has its own setting, `workspaces.whoCanCreate`.

`POST /api/v1/invites` takes an email, an optional `workspace` (slug or id of a shared workspace) with a workspace-wide `role` (`admin`, `write` or `read`, default `read`), and `sendEmail`. The response always carries the link `PUBLIC_URL/invite/<token>`. The server stores only a hash of the token, so this response is the one time the link is shown. If email is configured and `sendEmail` is true, the server also emails the link. `emailAvailable` and `emailSent` tell the caller what happened, so a client can hide its email option when `emailAvailable` is false. An email address that already has an account returns 409, and the message suggests adding the user to the workspace instead.

An invite expires after 7 days and works once. `GET /api/v1/invites` lists invites (all of them for the root user, the ones you sent for anyone else). `DELETE /api/v1/invites/{id}` revokes one. `POST /api/v1/invites/{id}/resend` revokes the old link and issues a new one.

The invitee opens the link. `GET /api/v1/invites/{token}` returns only the invited email and the workspace name, and `POST /api/v1/invites/{token}/accept` with a password and an optional display name creates the user, their personal workspace and the membership in one step, then signs them in. Unknown, expired, revoked and used tokens all return the same 404. These two routes need no sign-in and are rate limited. The page at `/invite/<token>` is part of the dashboard.

## Users and password resets

The root user lists accounts with `GET /api/v1/admin/users` and disables or enables one with `PATCH /api/v1/admin/users/{id}` and `{"disabled":true}`. A disabled user cannot sign in, and their sessions and API keys stop working at once. The root user cannot disable themselves, and the last root user cannot be disabled. There is no way to delete a user.

Anyone can ask for a reset link with `POST /api/v1/auth/password-reset/request`. The response is always the same, whether or not the account exists. The server sends the link only when email is configured and the account exists and is enabled. `POST /api/v1/auth/password-reset/confirm` with the token and a new password sets the password. A link works once and expires after an hour. A reset ends all of the user's sessions and keeps their API keys. Both routes are rate limited per IP address, and the request route per email address too.

Without email, the root user generates a link with `POST /api/v1/admin/users/{id}/password-reset`. The response shows the link once, and the root user passes it on.

## Agent sign-in

An agent that has no credentials can ask the user for access with an email code, when the instance has email:

1. `POST /api/v1/auth/agent/request-code` with `{"email":"alex@example.com"}`. The response is the same for every address. The server emails an 8-digit code to an existing, enabled account only.
2. The user reads the code from the email and gives it to the agent.
3. `POST /api/v1/auth/agent/verify-code` with `{"email":"alex@example.com","code":"12345678","label":"my agent"}` returns a new API key once, with the full access of that user. `label` defaults to `agent`. The 50 active keys per user limit applies.

A code works once, expires after 10 minutes and allows 5 attempts. Requests are limited to 3 per hour per email address, plus limits per IP address and for the whole instance.

When the instance cannot send email, `request-code` returns 409 `email_not_configured`. The message tells the agent that the user must create an API key in the dashboard under Settings, API keys, and give it to the agent.

### Where agents keep the key

Agents store the key in `~/.selfhost/credentials` with mode 0600 (the directory with mode 0700), as JSON with the instance URL and the key:

```json
{"url": "https://sites.example.com", "key": "sk_..."}
```

An agent reads that file at the start of a session before it asks the user for anything, and sends the key as `Authorization: Bearer <key>`. The `verify-code` response repeats the path in `credentialsPath`. The directory name comes from one constant, `AgentCredentialsDir` in `internal/config/agent.go`, so renaming the project changes one line.

## Configuration

The server reads these environment variables.

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Address the HTTP server listens on, as `host:port` or `:port`. |
| `DATA_DIR` | `./data` (`/data` in the Docker image) | Directory for the SQLite database file `selfhost.db`, published site files under `sites/`, upload temporaries under `tmp/` and the generated `secret.key`. Keep it on a persistent volume. |
| `DATABASE_URL` | unset | PostgreSQL connection URL. When unset, the server uses SQLite in `DATA_DIR`. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. Logs are JSON on standard output. |
| `SENTRY_DSN` | unset | Enables error reporting through the Sentry SDK. A self-hosted GlitchTip works. |
| `API_KEY_PREFIX` | `sk_` | Constant prefix of generated API keys, 1 to 16 letters, digits or underscores. Changing it invalidates existing keys. |
| `PUBLIC_URL` | `http://localhost` plus the port of `LISTEN_ADDR` | Address of the dashboard and API, such as `https://sites.example.com`: scheme, host and optional port, no path. The scheme decides whether cookies are Secure and which scheme the wildcard check uses. The host is the management host and the default `hosting.baseDomain`. |
| `SECRET_KEY` | unset | 32 random bytes, base64 encoded, used to encrypt stored secrets. When unset, the server uses `DATA_DIR/secret.key` and creates it on first start. See [Secrets at rest](#secrets-at-rest). |
| `TRUSTED_PROXIES` | unset | Comma-separated IP addresses or CIDR ranges of reverse proxies. The server reads `X-Forwarded-For` and `X-Forwarded-Proto` only from these peers. |
| `TLS_ENABLED` | `false` | Built-in HTTPS. See [Built-in HTTPS](#built-in-https) for this and the six variables below. |
| `TLS_HTTP_ADDR` | `:80` | Challenge and redirect listener when built-in HTTPS is on. |
| `TLS_HTTPS_ADDR` | `:443` | HTTPS listener when built-in HTTPS is on. It replaces `LISTEN_ADDR`. |
| `ACME_EMAIL` | unset | ACME account contact. Required when built-in HTTPS is on. |
| `ACME_CA` | Let's Encrypt production | ACME directory URL. |
| `ACME_CA_ROOT` | unset | PEM file with the root certificate of a private ACME server. |
| `TLS_STORAGE` | `DATA_DIR/certs` | Account key and certificate directory. |

`.env.example` lists the same keys with fake values. The server does not load `.env` files itself, so export the variables or pass them to Docker.

## Run with Docker

```sh
docker build -t selfhost .
docker run -p 8080:8080 -v selfhost-data:/data selfhost
```

For HTTPS without a proxy, publish ports 80 and 443 and set the variables from [Built-in HTTPS](#built-in-https).

To use PostgreSQL, add `-e DATABASE_URL=postgres://user:password@host:5432/dbname`.

## Run from source

You need the Go version in `go.mod`, and Node 22 with pnpm to build the dashboard.

```sh
pnpm --dir web install
pnpm --dir web build
go run ./cmd/selfhost
```

## Development

```sh
gofmt -l .
go vet ./...
golangci-lint run
go test ./...
pnpm --dir web lint
pnpm --dir web typecheck
pnpm --dir web test
```

`go test` runs the database tests against SQLite. To run them against PostgreSQL as well, set `TEST_DATABASE_URL` to a test database. The store tests drop the tables they create. The auth and API tests create a schema of their own per test and drop it afterwards.

## Licence

MIT. See `LICENSE`.
