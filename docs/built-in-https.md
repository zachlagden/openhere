# Built-in HTTPS

The server can terminate HTTPS itself and obtain its own certificates from an ACME certificate authority, so it runs without a reverse proxy. It is off by default. Behind a reverse proxy, leave `TLS_ENABLED` unset: the proxy owns TLS and nothing in this document applies. See [Reverse proxy](reverse-proxy.md).

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

## Which names get a certificate

1. **The management host** (the host of `PUBLIC_URL`) gets a certificate when the server starts and renews it on its own.
2. **Verified custom domains** get a certificate the first time a visitor connects, through on-demand issuance. The server issues one only when `domains.Service.Lookup` reports the name as a verified custom domain and `hosting.customDomainsEnabled` is on. Every other name is refused before any ACME request, so a visitor cannot make the server contact the CA by sending an arbitrary host name. Refused names are never logged individually. The server logs a count of refusals at debug level at most once a minute. The server also limits the requests that reach the CA: 5 per name, 10 per site and 50 in total per hour. Only an attempt to contact the CA counts. A decision to refuse a name, or a handshake that finds a stored certificate, uses none of the quota. The per-site limit stops one site with many verified names from using up the total. These limits keep the server within the CA's limits. Removing a domain or switching the feature off stops renewal.
3. **Site subdomains** use one wildcard certificate for `*.{baseDomain}`. The server never requests a certificate for a single subdomain name, because that would let any visitor trigger issuance. A subdomain name without a wildcard certificate gets a failed TLS handshake.

## Wildcard certificates for subdomain mode

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

## Security headers and cookies

Over built-in HTTPS the server sends `Strict-Transport-Security: max-age=31536000` on the management host, on custom domains and on subdomain-mode sites. It sets no `includeSubDomains` and no `preload`, so each host only commits itself. It sends no HSTS over plain HTTP or when built-in HTTPS is off, because a reverse proxy may add its own. A request that arrives on the TLS listener counts as HTTPS for cookies without `TRUSTED_PROXIES`, so the session cookie is `__Host-session` and Secure.

## Port 80 and the unauthenticated surface

Built-in HTTPS adds no unauthenticated API route. The ACME challenge path `/.well-known/acme-challenge/{token}` is answered by the certificate library on the port-80 listener before the redirect, and only for tokens the server itself issued. The API router never sees port 80. The TLS-ALPN challenge is answered inside the TLS handshake on `TLS_HTTPS_ADDR`.

## Status

Root sees `detail.tls` in `GET /api/v1/hosting/status`:

- `enabled` is true when the server runs with `TLS_ENABLED`.
- `managementCertificateExpiresAt` is the expiry of the management host certificate.
- `wildcard.state` is `none`, `pending`, `ok` or `failed`, with a short `reason` for `none` and `failed`. The full error is in the server log.
- `customDomains.count` and `customDomains.soonestExpiresAt` cover verified custom domains with a loaded certificate.
- `dnsProviderConfigured` is true when a provider and a token are stored.
