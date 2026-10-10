# selfhost

A self-hosted, open source alternative to here.now: hosting for already built static sites, aimed at AI agents, with a dashboard and a REST API. It targets one person or a small team running it on a VPS. It is not a public sign-up service.

"selfhost" is a placeholder name and will change.

## Status

Early development. The server starts, loads its configuration, connects to the database, applies migrations and serves the authentication API (first-time setup, login, sessions and API keys), the workspace API (workspaces, members, folders and sites), the publishing API (versions, uploads, finalize, restore) and the instance settings API. It also serves published sites by subdomain, by path or at custom domains the owner proves they control, and gates them with per-site access modes (public, password or restricted). It sends email through SMTP or Resend (see [Email](#email)) and builds on that for invites, password resets, agent sign-in and the viewer email code. There is no dashboard yet.

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

`docs/openapi.yaml` describes every endpoint (OpenAPI 3.1). The server does not serve it yet. Only `GET /api/v1/setup/status`, `POST /api/v1/setup`, `POST /api/v1/auth/login`, `GET /api/v1/invites/{token}`, `POST /api/v1/invites/{token}/accept`, `POST /api/v1/auth/password-reset/request`, `POST /api/v1/auth/password-reset/confirm`, `POST /api/v1/auth/agent/request-code` and `POST /api/v1/auth/agent/verify-code` work without credentials, plus `GET /api/v1/access/handoff`, which redirects a caller without credentials to `/login`. Every other path returns a JSON 401, or a JSON 404 once signed in.

- Browsers sign in with a session cookie (HttpOnly, SameSite=Lax, 30 days, renewed on use). Over HTTPS it is named `__Host-session`, is Secure, has Path=/ and no Domain attribute, so a sibling subdomain cannot overwrite it. Over plain HTTP (local use) it is named `session`. The server reads only the form that matches the request. State-changing requests that use the cookie must send an `X-Requested-With` header and an `Origin` header that matches the host.
- Scripts and agents use `Authorization: Bearer <key>` with a key from `POST /api/v1/me/keys`. The response shows the full key once. The server stores only its SHA-256 hash. Each user can hold 50 active keys.
- Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. `retry_after` appears on 429 responses.

### Rate limits

Login and setup are limited per IP address and per email: login allows 30 attempts per IP and 10 per email in 15 minutes, setup allows 10 per IP and 5 per email. The account sign-in handoff allows 30 calls per user per minute. Checking a custom domain allows 20 calls per user per minute, and claiming, changing or deleting domains allows 30 per user per minute. The counters live in process memory, so a restart resets them and each server process counts separately. A successful login resets that email's counter. The routes that work without credentials and can send email or issue credentials have their own limits per IP address and, where an email is involved, per email address: invite lookup 60 per minute and invite accept 10 per 15 minutes per IP, reset requests 5 per 15 minutes per IP and 3 per hour per email, reset confirmation 10 per 15 minutes per IP, agent code requests 10 per 15 minutes per IP, 3 per hour per email and 100 per hour for the whole instance, agent code verification 20 per 15 minutes per IP and 10 per 15 minutes per email. Creating or resending invites allows 30 per hour per user. The viewer gate has its own limits, listed under [Access control](#access-control).

### Running behind a reverse proxy

Set `TRUSTED_PROXIES` to the addresses of your proxy. The server then reads the client address from `X-Forwarded-For` and decides whether a request used HTTPS from `X-Forwarded-Proto`, but only for requests that arrive from those addresses. Without it, every client appears to be the proxy and shares one rate limit. The proxy must pass the original `Host` header, because the CSRF check compares it with `Origin` and the host router needs it to find the site.

Set `PUBLIC_URL` to the address people use for the dashboard and API, for example `https://sites.example.com`. An `https://` value makes the session cookie Secure and `__Host-` prefixed, and tells the wildcard check which scheme to probe.

For subdomain mode the proxy needs two more things. Create a wildcard DNS record, such as `*.sites.example.com`, that points to the server, and obtain a wildcard certificate for `*.sites.example.com` (DNS-01 validation) if the sites use HTTPS. Route every host under the wildcard to this server and keep the `Host` header.

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

`GET /api/v1/hosting/status` gives any signed-in user the instance view: the state and reason of each mode, notices, and `degraded`, which is true while subdomain mode is on and its last wildcard check failed. The root user also gets a `detail` object with the check status, when it last ran, the failure text, the consecutive failure count and the reason subdomain mode was switched off. Other users never get `detail`.

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

### Behind a reverse proxy

The server does not obtain certificates for custom domains. The reverse proxy in front of it must route each custom domain's host to this server with the `Host` header intact, and must hold a certificate for each one. A wildcard certificate for the base domain does not cover a custom domain. With Traefik (including the one Coolify manages) or Caddy, add the custom domain as a host that the proxy routes to this server. Whether the proxy requests a certificate for it on its own depends on how you configured it, so check your proxy's documentation. Built-in TLS for custom domains is planned and is not part of this version.

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

The account link goes to `GET /api/v1/access/handoff?site=<slug>&return=<path>` on the management host. On a custom domain it also carries `host=<domain>`, and the redirect goes back to that domain. A caller with a session or API key and read access gets a single-use token, valid for 60 seconds and for that site only, and a redirect to the site host, which exchanges it for the viewer cookie and redirects to `return`. A caller without credentials is redirected to `/login?next=...`. **The login page ships with the dashboard, so until then the interactive path works only for users who already have a session cookie in the browser.** The return path must be a path on the site: absolute URLs, protocol-relative URLs, backslashes, control characters, fragments and dot segments are refused, including percent-encoded forms.

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

The SMTP password and the Resend API key are write-only. The settings API never returns them, only whether one is set. The server encrypts them with AES-256-GCM before it stores them. The key comes from `SECRET_KEY` (32 random bytes, base64 encoded) when set. Otherwise the server creates `secret.key` in `DATA_DIR` with mode 0600 on first start and reuses it. Generate a key with `openssl rand -base64 32`. Back up the key with the database: without it, stored secrets cannot be read, and email stays off until the root user enters them again. The server derives the agent sign-in code hashes from the same key, so a changed key also invalidates codes that are in flight.

## Invites

An invite creates an instance account. Nobody can sign up on their own. By default only the root user can invite. Set `invites.whoCanInvite` to `workspaceAdmins` to let a workspace admin invite into a workspace they administer. Creating shared workspaces has its own setting, `workspaces.whoCanCreate`.

`POST /api/v1/invites` takes an email, an optional `workspace` (slug or id of a shared workspace) with a workspace-wide `role` (`admin`, `write` or `read`, default `read`), and `sendEmail`. The response always carries the link `PUBLIC_URL/invite/<token>`. The server stores only a hash of the token, so this response is the one time the link is shown. If email is configured and `sendEmail` is true, the server also emails the link. `emailAvailable` and `emailSent` tell the caller what happened, so a client can hide its email option when `emailAvailable` is false. An email address that already has an account returns 409, and the message suggests adding the user to the workspace instead.

An invite expires after 7 days and works once. `GET /api/v1/invites` lists invites (all of them for the root user, the ones you sent for anyone else). `DELETE /api/v1/invites/{id}` revokes one. `POST /api/v1/invites/{id}/resend` revokes the old link and issues a new one.

The invitee opens the link. `GET /api/v1/invites/{token}` returns only the invited email and the workspace name, and `POST /api/v1/invites/{token}/accept` with a password and an optional display name creates the user, their personal workspace and the membership in one step, then signs them in. Unknown, expired, revoked and used tokens all return the same 404. These two routes need no sign-in and are rate limited. The page at `/invite/<token>` belongs to the dashboard, which does not exist yet.

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
