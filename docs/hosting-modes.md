# Hosting modes

The server serves each site's live version at up to three kinds of address. A public site needs no sign-in, and the other [access modes](access-control.md) put a gate in front of the files.

| Mode | Address | Setting |
|---|---|---|
| Subdomain | `https://{slug}.{baseDomain}/` | `hosting.subdomainEnabled`, off until a wildcard check passes |
| Path | `{PUBLIC_URL}/s/{slug}/` | `hosting.pathEnabled`, on by default |
| Custom domain | `https://{your domain}/` | `hosting.customDomainsEnabled`, on by default. See [Custom domains](#custom-domains) |

Every site response and every publish response carries a `url` field with the address the instance serves the site at right now: the primary custom domain when the site has one, else the subdomain address when subdomain mode is on, else the path address when path mode is on, else the first verified custom domain by name, else `null`.

## Which mode to use

Subdomain mode and custom domains give each site its own browser origin. The site's JavaScript can use cookies and `localStorage` and cannot touch the dashboard. Path mode puts sites on the dashboard's origin, so the server sandboxes them (see below). Prefer subdomain mode, or custom domains, for anything with scripts that need storage, and for any site behind a password or an allowlist.

A new instance starts with path mode on and subdomain mode off, because subdomain mode needs wildcard DNS that the server cannot create. Path mode works with no DNS setup beyond the dashboard's own name.

## Path mode and its sandbox

Path mode serves a site under `/s/{slug}/` on the dashboard host. To stop hosted JavaScript from acting as a signed-in user, every `/s/` response, including errors and redirects, carries:

```
Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups allow-downloads allow-modals allow-presentation
```

The policy omits `allow-same-origin`, so the page runs in an opaque origin. The consequences:

- Scripts run, but they cannot read cookies or `localStorage` and cannot call the API as the signed-in user.
- Responses carry `Access-Control-Allow-Origin: *`, so module scripts and fonts load from the opaque origin.
- The page must use relative links. A link such as `/style.css` points at the dashboard host, not at the site.
- A gated site needs a `SameSite=None` viewer cookie in this mode. Over plain HTTP the server shows an HTTPS-required page instead. A browser that blocks third-party cookies can withhold the cookie from the page's own subresources. See [Access control](access-control.md#viewer-cookies).

Subdomain-mode sites and custom domains get no sandbox policy, because the browser already separates their origin from the dashboard.

## Choosing a base domain

`hosting.baseDomain` defaults to the host of `PUBLIC_URL`, so with the dashboard at `sites.example.com` the sites live at `{slug}.sites.example.com`. That works, with two costs.

Tenant pages and the dashboard then share a registrable domain. A page on `{slug}.sites.example.com` can set a cookie with `Domain=sites.example.com`, and the browser sends that cookie to the dashboard host. The dashboard's session cookie is `__Host-session` over HTTPS, which no sibling host can overwrite, but a sibling can still send cookies the dashboard does not expect, such as very large ones that make requests fail. Over plain HTTP the session cookie is `session` and has no such protection.

A certificate for `*.example.com` does not cover `{slug}.sites.example.com`, because a wildcard covers one label. Subdomain mode under a subdomain needs its own wildcard for `*.sites.example.com`.

The cleaner setup is a separate registrable domain for tenants, such as `example-sites.net`, with the dashboard on `sites.example.com`. Set `hosting.baseDomain` to `example-sites.net`, point `*.example-sites.net` at the server and obtain a wildcard certificate for it. Cookies set on tenant hosts can then never reach the dashboard. Sibling tenant sites still share the base domain, which only the browser's public suffix list can separate.

### Hosting under a subdomain behind a CDN

Many CDNs issue a free certificate for the apex and one wildcard level only. Cloudflare's free universal certificate covers `example.com` and `*.example.com`, so a proxied (orange cloud) record for `*.artifacts.example.com` presents a certificate that does not match, and browsers show a warning. There are three ways out:

1. Use path mode and skip subdomain mode. Nothing about the CDN changes.
2. Use a separate registrable base domain whose own apex wildcard the CDN covers.
3. Point the wildcard record `*.artifacts.example.com` at the server as DNS-only (grey cloud) and let the server's [built-in HTTPS](built-in-https.md#wildcard-certificates-for-subdomain-mode) obtain the `*.artifacts.example.com` certificate through a DNS-01 challenge.

The behaviour of a particular CDN's certificates is that provider's to define. Check its documentation before you rely on this.

## Routing

The server normalises the request host (it drops the port and a trailing dot and lower-cases it) and then decides:

- The host of `PUBLIC_URL` is the management host. It serves the API, and `/s/{slug}/...` serves a site in path mode. `/s/{slug}` redirects to `/s/{slug}/`. A site's files are never reachable on the management host except through `/s/`.
- `{slug}.{baseDomain}` with subdomain mode on serves that site. The slug must be a single DNS label.
- A host that is a verified custom domain, while `hosting.customDomainsEnabled` is on, serves the site that owns it exactly as subdomain mode does. The router checks the management host first, then the subdomain pattern, then the custom domains.
- Every other host serves nothing from the API. Site hosts never run API authentication: they ignore session cookies and API keys. A gated site has its own viewer cookie, described under [Access control](access-control.md).

An unknown host, an unverified custom domain, a custom domain while the feature is off, an unknown slug, a disabled mode and a site without a live version all return the same 404 response with the same headers and a one-line body, so a visitor cannot tell them apart. Only `GET` and `HEAD` work on site hosts and under `/s/`, apart from the `POST` forms of the gate on gated sites. Other methods return 405 with `Allow: GET, HEAD`.

`hosting.baseDomain` defaults to the host name of `PUBLIC_URL`. Changing it to a different value resets the wildcard check and switches subdomain mode off. If that would leave both modes off, the server switches path mode on in the same change and the response carries a notice with the code `path_mode_enabled`.

At least one mode stays on. A settings change that would leave `hosting.subdomainEnabled` and `hosting.pathEnabled` both false returns 409 `no_hosting_mode`. The server judges the stored values merged with the patch, so one patch can switch one mode on and the other off.

## What users see

Every site response and publish response has a `hosting` object:

- `reachable` is true when the site has a live version and at least one mode can serve it now. A site with a password or restricted access does not count path mode over plain HTTP, because the gate needs a secure connection there. A verified custom domain counts while custom domains are enabled, even when subdomain and path mode are off for the site.
- `url` is the same address as the top-level `url`.
- `modes.subdomain`, `modes.path` and `modes.customDomains` each have a `state` (`on` or `off`) and a `reason`. The reason is `switched_off`, `wildcard_check_failed`, `https_required` or `not_applicable`, and null when the mode is on and healthy.
- `notices` lists `{code, message}` entries in plain language, such as "The subdomain address for this site is switched off.", "Nothing has been published to this site yet." and "A custom domain is waiting for DNS verification.". They carry no administrator detail.

Every user with read access to a site gets the same `hosting` object, root or not. A user sees nothing about sites they cannot read.

`GET /api/v1/hosting/status` gives any signed-in user the instance view: the state and reason of each mode, notices, and `degraded`, which is true while subdomain mode is on and its last wildcard check failed. The root user also gets a `detail` object with the check status, when it last ran, the failure text, the consecutive failure count, the reason subdomain mode was switched off and a `tls` object with the certificate state (see [Built-in HTTPS](built-in-https.md)). Other users never get `detail`.

## Which file a path serves

For a request path `/p`, the server tries `p`, then `p.html`, then `p/index.html`. `/` serves `index.html`. A path with a trailing slash (`/p/`) tries `p/index.html`, then `p.html`. When only `p/index.html` exists, `/p` redirects once to `/p/`, which keeps relative links working. Next.js static exports (`about.html`, `docs/index.html`, `_next/...`) work as built. When nothing matches, the server serves the site's `404.html` with status 404 if it exists, and the plain 404 otherwise. SPA mode comes before `404.html` for paths without a file extension.

Responses support `Range`, `If-None-Match` and `If-Modified-Since`. The `ETag` is the stored SHA-256. The content type is the one stored at publish time (guessed from the extension when the manifest left it empty), and the server never sniffs it. `X-Content-Type-Options: nosniff` is always set. HTML responses send `Cache-Control: no-cache` and every other file sends `Cache-Control: public, max-age=3600, must-revalidate`. A new version is live as soon as finalize or restore returns. The server never serves a symbolic link or anything outside the version's directory.

## Wildcard check

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

With built-in HTTPS on, the server requests a certificate for each verified custom domain the first time a visitor connects. See [Built-in HTTPS](built-in-https.md).

Behind a reverse proxy, the proxy owns TLS. It must route each custom domain's host to this server with the `Host` header intact, and it must hold a certificate for each one. A wildcard certificate for the base domain does not cover a custom domain. With Traefik (including the one Coolify manages) or Caddy, add the custom domain as a host that the proxy routes to this server. Whether the proxy requests a certificate for it on its own depends on how you configured it, so check your proxy's documentation.

### Ownership rules

- Several sites can hold a pending claim on one name, each with its own challenge token. The first site whose TXT record verifies takes the name, and the server deletes every competing pending claim in the same step. A pending claim that has not been verified after 7 days is released by the maintenance task. A verified domain is released only by deleting it, and nobody can claim a verified name. Claiming a name never shows which site holds a claim on it.
- An unverified domain never serves. A request for it gets the same 404 as an unknown host.
- The maintenance task re-checks every verified domain once a day. A lookup that succeeds without the token, or finds no record, counts as a miss. After 3 consecutive daily misses the domain goes back to `pending_verification`, stops serving and stops being primary. The site and its other domains are untouched. A timeout or resolver error is not a miss, so a DNS outage never takes domains down. The domain keeps `lastCheckedAt` and `lastCheckResult` for the last lookup, and the 7-day clock for pending claims restarts when a domain drops.
- Verified domains work for gated sites. The gate page on a custom domain sends the sign-in link with a `host` parameter, and `GET /api/v1/access/handoff` returns to that domain only if it is a verified domain of the same site. The viewer cookie is host-only and named like the subdomain-mode cookie (`__Host-viewer` when the request arrived over HTTPS, `viewer` otherwise).

`hosting.customDomainsEnabled` (default true) turns serving off for every custom domain at once without touching stored domains. `hosting.maxDomainsPerSite` (default 10, 1 to 50) caps the domains one site holds. Changing `hosting.baseDomain` to a name that equals or contains a custom domain returns 409 `base_domain_in_use`.
