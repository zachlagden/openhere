# Running behind a reverse proxy

You can let the server terminate HTTPS itself (see [Built-in HTTPS](built-in-https.md)) or put a reverse proxy in front of it. This document covers the proxy case. Leave `TLS_ENABLED` unset: the proxy owns TLS, and the server listens on plain HTTP at `LISTEN_ADDR`.

Statements about the server come from its code and tests. Statements about Traefik, Caddy, Coolify and Cloudflare come from their official documentation, fetched when this page was written, and the ones marked **unverified** come from no source I could check. Test your own setup before you rely on it.

## What the server needs

1. **`PUBLIC_URL`** set to the address people use, for example `https://sites.example.com`. An `https://` value makes the session cookie `Secure` and `__Host-` prefixed and tells the wildcard check which scheme to probe. Links in emails start with it.
2. **The original `Host` header.** The server routes by `Host`: the host of `PUBLIC_URL` is the dashboard and API, `{slug}.{baseDomain}` is a site and a verified custom domain is a site. The CSRF check also compares `Origin` with `Host`. If the proxy rewrites `Host` to the upstream address, the dashboard stops working and sites return 404.
3. **`TRUSTED_PROXIES`** set to the address or range the proxy connects from, as the server sees it. The server then reads the client address from `X-Forwarded-For` and decides whether a request used HTTPS from `X-Forwarded-Proto`, but only for connections from those addresses. It takes the right-most `X-Forwarded-For` entry that is not itself a trusted proxy. Without the setting, every client appears to be the proxy and shares one rate limit, and analytics sees one visitor.
4. **`X-Forwarded-For` and `X-Forwarded-Proto`** set by the proxy, with any value from the client discarded or appended to rather than trusted. Both proxies below do this by default.

Do not set `TRUSTED_PROXIES` to a range that clients can reach. A client in that range could then forge its address.

The server does not warn at startup when `X-Forwarded-For` or `X-Real-IP` arrives from an address outside `TRUSTED_PROXIES`, and it ignores those headers. Check your setup once: if every client shares one rate limit, or the analytics show one visitor, the proxy's address is missing from `TRUSTED_PROXIES`.

## Which names the proxy must route

| Mode | Hosts to route to the server |
|---|---|
| Path mode only | The host of `PUBLIC_URL` |
| Subdomain mode | The host of `PUBLIC_URL`, plus every host under `*.{baseDomain}` |
| Custom domains | Each verified custom domain, one by one |

**In this mode, certificates are the proxy's job.** The server obtains certificates only with built-in HTTPS. Behind a proxy, the proxy must hold a certificate for the dashboard host, a wildcard certificate for `*.{baseDomain}` when you use subdomain mode, and a certificate for each custom domain. A wildcard certificate needs a DNS-based challenge, so the proxy needs credentials for your DNS provider. A wildcard for `*.example.com` does not cover `{slug}.sites.example.com`. See [Hosting modes](hosting-modes.md#choosing-a-base-domain).

The server cannot tell the proxy which custom domains exist. It has no unauthenticated endpoint that a proxy could query to ask whether a name is allowed, so on-demand certificate issuance in the proxy cannot be restricted by asking the server. Add each custom domain to the proxy yourself after you verify it.

After you set up subdomain routing, run the wildcard check from the dashboard (Admin, Hosting). It resolves a random name under the base domain and requests a one-time token through your DNS, proxy and certificate, so a pass proves all three.

## Traefik

Traefik forwards the client `Host` header to the backend by default (`passHostHeader` defaults to true, per the Traefik documentation). It trusts `X-Forwarded-*` headers from a client only when the client address is in the entry point's `forwardedHeaders.trustedIPs`. The `insecure` option trusts them from everyone, and the documentation recommends it for tests only.

A router that sends the dashboard host and all hosts under the base domain to this server uses the `Host` matcher, which supports a wildcard subdomain, or a `HostRegexp` matcher:

```
Host(`sites.example.com`) || HostRegexp(`^.+\.sites\.example\.com$`)
```

The Traefik documentation recommends `Host` with a wildcard over `HostRegexp` when it fits, because it allows attaching TLS options.

Set `TRUSTED_PROXIES` to the address Traefik connects from. When Traefik runs as a container, that is an address on the Docker network the two share, so use that network's range and not `0.0.0.0/0`.

## Coolify

Coolify runs Traefik. Per its documentation, it generates a Traefik route from each application's domain and container port and configures automatic TLS for HTTPS domains with a Let's Encrypt resolver that uses the HTTP challenge, on ports 80 and 443.

- Build the application from the project's Dockerfile and expose container port 8080. The image listens on `:8080`.
- Put the dashboard domain in the application's domain field, as `https://sites.example.com`, and set `PUBLIC_URL` to the same value.
- Set `TRUSTED_PROXIES` to the range of the Docker network that Coolify's proxy uses. Use the Docker network's actual range and not a wide private range, because other containers on a wider range could forge addresses.
- A wildcard certificate for subdomain mode needs a DNS challenge. Coolify's overview page does not describe how to configure one for an application, so **unverified**: follow Coolify's own wildcard-certificate guide. Whether one application can list a wildcard host and several custom domains in its domain field is **unverified** here as well.
- If wildcard certificates are a problem on your Coolify setup, path mode needs only the dashboard host.

## Cloudflare in front of Traefik or Coolify

**Symptom.** Cloudflare proxies the host and Traefik (Coolify's proxy) sits in front of the server. Every request reaches the server with `X-Forwarded-For` set to a Cloudflare edge address, because Traefik replaces the header with the address it received the connection from. The real client appears only in `Cf-Connecting-Ip`. The server then keys rate limits, login lockouts, gate limits, analytics visitor hashes and location lookups on Cloudflare edge addresses. One attacker spreads across several limiter keys, and unrelated visitors share keys.

**Fix.** Set both of these:

```
TRUSTED_PROXIES=172.18.0.0/16
CLIENT_IP_HEADER=CF-Connecting-IP
```

Use your own proxy range in place of the example. `CLIENT_IP_HEADER` names one request header that carries the client address. The server reads it only when the connection comes from a `TRUSTED_PROXIES` address, and it ignores the header from every other peer. The header must hold exactly one value that is a bare IP address. A port, a list, a hostname, an IPv6 zone, a second header line or an empty value makes the server fall back to `X-Forwarded-For` for that request. `CLIENT_IP_HEADER` cannot name `X-Forwarded-For`, `X-Forwarded-Proto` or `Forwarded`. Leave it unset to keep the `X-Forwarded-For` behaviour described above. `X-Forwarded-Proto` handling does not change.

**Security note.** The server trusts the header only from `TRUSTED_PROXIES`, but that does not make the value reliable. Cloudflare sets `Cf-Connecting-Ip` on traffic from its edge to your origin. A client that reaches Traefik without passing through Cloudflare can send any `Cf-Connecting-Ip` it likes, and Traefik will pass it on. That client could then pick its own rate limit key and evade lockouts. So make the origin accept connections only from Cloudflare:

- Restrict ports 80 and 443 on the firewall, or at the proxy, to the ranges Cloudflare publishes at [cloudflare.com/ips](https://www.cloudflare.com/ips/). Cloudflare's [IP address documentation](https://developers.cloudflare.com/fundamentals/concepts/cloudflare-ip-addresses/) recommends blocking traffic that does not come from those ranges, because anyone who finds the origin address can otherwise reach the server directly. It says the ranges change rarely and describes an API for keeping a configuration up to date.
- Optionally add [Authenticated Origin Pulls](https://developers.cloudflare.com/ssl/origin-configuration/authenticated-origin-pull/). Cloudflare describes it as an extra check that requests to the origin come from the Cloudflare network. The global form confirms only that a request came from Cloudflare's network, not from your account. The per-zone and per-hostname forms use your own certificate and are stricter.

An alternative is to configure Traefik's `forwardedHeaders.trustedIPs` on the entry point with Cloudflare's ranges. Traefik then keeps the `X-Forwarded-For` and `X-Forwarded-Proto` values that arrive from those addresses, and Cloudflare appends the client address to `X-Forwarded-For`. This is a global proxy change: it applies to every router on that entry point and every application behind it, and in Coolify it means editing the proxy configuration. The server then also needs Cloudflare's ranges in `TRUSTED_PROXIES`, because it takes the right-most `X-Forwarded-For` entry that is not a trusted proxy. That makes a wide range trusted, so it needs the same firewall restriction. This page does not cover the details of the Traefik configuration.

## Cloudflare Rocket Loader

Rocket Loader defers a page's JavaScript until after rendering. It rewrites script tags and injects an inline script. The dashboard's strict Content-Security-Policy blocks that inline script, so the dashboard fails to load. Turn Rocket Loader off for the dashboard host, either with a Configuration Rule scoped to the hostname or with the zone setting under Speed, Optimization. Cloudflare's [Rocket Loader page](https://developers.cloudflare.com/speed/optimization/content/rocket-loader/) says to disable it and retest when scripts misbehave, and that a domain with a Content-Security-Policy must update its headers to support it. The page does not describe Configuration Rules, so check the rule options in your dashboard.

## Cloudflare and site files

A live test of this setup (Cloudflare, Traefik and the server in path mode) showed these effects. Cloudflare's behaviour belongs to Cloudflare and can change, so check its current documentation before you rely on a detail.

- **Cache headers.** The server sends `Cache-Control: no-cache` for site files except content-hashed ones (see [Caching](hosting-modes.md#caching)). Cloudflare treated those responses as uncacheable, so a republish shows at once. A file sent with a `max-age` stays at the Cloudflare edge until it expires, so a republish would not show until then. A cache rule or a Cloudflare setting can keep a file longer than the origin allows, so if a file looks stale at the edge, purge it and check the rules for that host.
- **Browser Cache TTL.** This zone setting raises the `max-age` that browsers see. The test saw `max-age=14400` in place of the origin value. Check the setting's options in Cloudflare's documentation for one that respects existing headers if you want the server's values to reach browsers.
- **Range requests.** Range requests on cacheable files returned the whole body with status 200 and no `Accept-Ranges` header. The same requests on `no-cache` files returned correct 206 responses. The server's `no-cache` policy avoids the problem for ordinary files.
- **Injected script.** Cloudflare's JavaScript detections add a small script to every HTML response. The test saw about 940 extra bytes, no `ETag` on HTML (pages then revalidate only with `If-Modified-Since`), and console errors from the script under the dashboard's strict Content-Security-Policy and inside sandboxed path-mode pages. To stop it for the host, look for JavaScript detections under Security, Bots in the Cloudflare dashboard and switch them off for the zone or for the host with a rule. I could not confirm the exact menu names, so follow Cloudflare's current documentation.
- **`/robots.txt`.** If you enable Cloudflare's managed `robots.txt`, Cloudflare answers `/robots.txt` on a proxied host, so a `robots.txt` file in a site is not served there.
- **`/healthz`, `/docs` and similar paths.** On the dashboard host, any path without a file extension that is not an API route returns the dashboard page with status 200, because the dashboard is a single-page app. A monitor that checks `/healthz` always sees 200. The server has no health route, because every route needs authentication.

## Caddy

Caddy keeps the original `Host` header when it proxies to an `http://` upstream, and it sets `X-Forwarded-For`, `X-Forwarded-Proto` and `X-Forwarded-Host` itself. It ignores incoming values of those headers unless the client is in its `trusted_proxies` list. A minimal Caddyfile for path mode, with Caddy on the same machine as the server:

```
sites.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

Run the server with `PUBLIC_URL=https://sites.example.com` and `TRUSTED_PROXIES=127.0.0.1`. Caddy obtains the certificate for `sites.example.com` automatically.

Let's Encrypt issues wildcard certificates only through the DNS challenge, per the Caddy documentation, so subdomain mode needs a Caddy site block for `*.sites.example.com` with a DNS challenge configured. How to configure the DNS provider module is Caddy's to define and is **unverified** here. Caddy's on-demand TLS needs an "ask" endpoint to limit issuance, and this server provides none, so list each custom domain in the Caddyfile.

## Checks

- `curl -sI https://sites.example.com/` returns the dashboard shell.
- The `url` of a published site matches the address you expect.
- The sign-in page works over HTTPS and the browser stores a `__Host-session` cookie.
- A site published in path mode loads at `https://sites.example.com/s/{slug}/`.
- In the dashboard, Admin, Hosting shows the wildcard check as passed when you use subdomain mode.
