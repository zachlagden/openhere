# Running behind a reverse proxy

You can let the server terminate HTTPS itself (see [Built-in HTTPS](built-in-https.md)) or put a reverse proxy in front of it. This document covers the proxy case. Leave `TLS_ENABLED` unset: the proxy owns TLS, and the server listens on plain HTTP at `LISTEN_ADDR`.

Statements about the server come from its code and tests. Statements about Traefik, Caddy and Coolify come from their official documentation, fetched when this page was written, and the ones marked **unverified** come from no source I could check. Test your own setup before you rely on it.

## What the server needs

1. **`PUBLIC_URL`** set to the address people use, for example `https://sites.example.com`. An `https://` value makes the session cookie `Secure` and `__Host-` prefixed and tells the wildcard check which scheme to probe. Links in emails start with it.
2. **The original `Host` header.** The server routes by `Host`: the host of `PUBLIC_URL` is the dashboard and API, `{slug}.{baseDomain}` is a site and a verified custom domain is a site. The CSRF check also compares `Origin` with `Host`. If the proxy rewrites `Host` to the upstream address, the dashboard stops working and sites return 404.
3. **`TRUSTED_PROXIES`** set to the address or range the proxy connects from, as the server sees it. The server then reads the client address from `X-Forwarded-For` and decides whether a request used HTTPS from `X-Forwarded-Proto`, but only for connections from those addresses. It takes the right-most `X-Forwarded-For` entry that is not itself a trusted proxy. Without the setting, every client appears to be the proxy and shares one rate limit, and analytics sees one visitor.
4. **`X-Forwarded-For` and `X-Forwarded-Proto`** set by the proxy, with any value from the client discarded or appended to rather than trusted. Both proxies below do this by default.

Do not set `TRUSTED_PROXIES` to a range that clients can reach. A client in that range could then forge its address.

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
