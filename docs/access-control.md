# Access control

Every site has one access mode. New sites start in the mode of the `sites.defaultAccessMode` setting, which is `public` unless you change it.

| Mode | Who gets in |
|---|---|
| `public` | Anyone with the link. |
| `password` | Anyone who knows the site's password. |
| `restricted` | Instance users who have at least read access to the site (through a workspace role or a grant scoped to the site or its folder), and viewers whose email address or email domain is on the site's allowlist. An empty allowlist means members only. |

Read and change the settings with `GET` and `PATCH /api/v1/publish/{slug}/access`. Reading needs read access to the site and changing needs write. `mode`, `password` (a string, or `null` to remove it), `emailAllowlist` and `domainAllowlist` can be sent together or alone. The lists replace the stored lists, each holds at most 500 entries, and a domain entry such as `example.com` matches addresses at exactly that domain. Password mode needs a password to exist. Passwords have 8 to 128 characters, are stored as argon2id hashes and are never returned or logged. Responses show `hasPassword` instead. `accessMode` also appears on every site.

Changing the mode, the password or either list moves the site's access epoch. Every viewer cookie carries the epoch it was issued under, so one change signs every viewer out. It also discards unused email codes and sign-in links for the site. Removing a user's workspace access takes effect at once too, because the server checks a member's access again on every request.

## What a visitor sees

A request for a gated site gets the gate page, an HTML page with status 401 and no site content. Requests that are clearly not page loads (scripts, images and `fetch` calls) get a plain 401. The gate runs before the server looks up any file, `404.html` included, so a path that exists and one that does not get the same answer. Gate pages are server-rendered with inline CSS, no JavaScript and no external requests.

- A **password** site shows a password form. A wrong password gets the same message whatever the reason. Unlock attempts are limited to 10 per IP and 100 per site in 15 minutes, and a limited request gets 429 with `Retry-After` (or a JSON `retry_after` for `Accept: application/json`).
- A **restricted** site shows a link that signs in an instance user, and, when the instance can send email, a form for allowlisted viewers. A viewer enters an email address and receives an 8-digit code that works once, for 10 minutes and for 5 attempts. The page answers the same way whether or not the address is on the allowlist, and only allowlisted addresses receive mail. Code requests are limited per address (3 per 10 minutes), per IP (10 per 15 minutes) and per site (60 per hour). Code checks are limited per address, per IP and per site as well.
- **Email codes need an email sender.** When the instance cannot send email, the access settings report `emailCodeAvailable: false`, the gate page says email sign-in is not available, and allowlisted addresses are stored but cannot sign in. Instance users can still use the account link. See [Email, invites and password resets](email-and-users.md).

The account link goes to `GET /api/v1/access/handoff?site=<slug>&return=<path>` on the management host. On a custom domain it also carries `host=<domain>`, and the redirect goes back to that domain. A caller with a session or API key and read access gets a single-use token, valid for 60 seconds and for that site only, and a redirect to the site host, which exchanges it for the viewer cookie and redirects to `return`. A caller without credentials is redirected to `/login?next=...`. The sign-in page is part of the dashboard. The return path must be a path on the site: absolute URLs, protocol-relative URLs, backslashes, control characters, fragments and dot segments are refused, including percent-encoded forms.

## Viewer cookies

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

## Routes the gate adds

On a site host, and under `/s/<slug>/` in path mode, a gated site answers four paths that need no credentials: `GET /.well-known/selfhost-auth` (exchanges a sign-in token) and `POST /.well-known/selfhost-gate/unlock`, `/code` and `/verify` (the gate forms). They exist only while the site is gated. On a public site they behave like any other path, and a site's own file at that path is shadowed only while the site is gated. The gate counters live in process memory, like the other rate limits.

## Trade-off: the gate page shows that a site exists

A gated site that has a live version answers with its gate page, so anyone who knows or guesses the slug can tell it exists. An unknown slug, and a site without a live version, still get the plain 404. The gate page shows nothing else about the site: no name, no description and no file names. If the existence of a slug is itself sensitive, choose slugs that cannot be guessed (the generated ones are).
