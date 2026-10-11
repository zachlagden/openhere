# Configuration

The server has two kinds of configuration. Environment variables are read once at startup and cover the things the process needs before it can open its database: where to listen, where to store data and how to reach PostgreSQL. Instance settings live in the database, are changed by the root user while the server runs and need no restart.

The server does not load `.env` files. Export the variables, pass them with `docker run -e` or put them in a systemd `EnvironmentFile`. `.env.example` lists every variable with fictional values.

## Environment variables

An empty value counts as unset. The server refuses to start, and names every problem it found, when a value is invalid.

| Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Address of the HTTP listener, as `host:port` or `:port`. Ignored when `TLS_ENABLED` is true. |
| `DATA_DIR` | `./data` (`/data` in the Docker image) | Directory for the SQLite file, published site files, upload temporaries, the GeoLite2 database, certificates and the generated `secret.key`. Keep it on a persistent volume. See [Upgrading and backups](upgrading-and-backups.md) for the layout. |
| `DATABASE_URL` | unset | `postgres://` or `postgresql://` URL with a host. When unset, the server uses SQLite at `DATA_DIR/openhere.db`. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. Logs are JSON on standard output. |
| `SENTRY_DSN` | unset | Sends errors to Sentry or a compatible service such as GlitchTip. Nothing is sent when unset. |
| `API_KEY_PREFIX` | `sk_` | Constant prefix of new API keys, 1 to 16 letters, digits or underscores. Changing it makes the server reject every existing key, because it checks the prefix before it looks a key up. |
| `PUBLIC_URL` | `http://localhost` plus the port of `LISTEN_ADDR` | Address people use for the dashboard and API: scheme, host and optional port, no path. Its host is the management host and the default `hosting.baseDomain`. An `https://` scheme makes cookies `Secure` and `__Host-` prefixed. Every link in an email starts with it. |
| `SECRET_KEY` | unset | 32 random bytes encoded as base64. Encrypts stored secrets. When unset, the server uses `DATA_DIR/secret.key` and creates it on first start. Generate one with `openssl rand -base64 32`. |
| `TRUSTED_PROXIES` | unset | Comma-separated IP addresses or CIDR ranges of reverse proxies. The server reads `X-Forwarded-For` and `X-Forwarded-Proto` only from these peers. See [Reverse proxy](reverse-proxy.md). |
| `ARGON2_CONCURRENCY` | `2` | Password hashes the server computes at once, 1 to 16. Each hash uses 64 MiB of memory while it runs. Further sign-ins, password changes and site password checks wait up to 5 seconds for a free slot, then get a 503 `server_busy` response with a `retry_after` value. Raise it on a server with spare memory and many simultaneous sign-ins. |
| `TLS_ENABLED` | `false` | Switches built-in HTTPS on. Accepts `true` or `false`. See [Built-in HTTPS](built-in-https.md). |
| `TLS_HTTP_ADDR` | `:80` | Challenge and redirect listener. Read only when `TLS_ENABLED` is true. |
| `TLS_HTTPS_ADDR` | `:443` | HTTPS listener. Read only when `TLS_ENABLED` is true. Must differ from `TLS_HTTP_ADDR`. |
| `ACME_EMAIL` | unset | Contact address of the ACME account. Required when `TLS_ENABLED` is true. |
| `ACME_CA` | Let's Encrypt production | URL of the ACME directory. |
| `ACME_CA_ROOT` | unset | Path to a PEM file with the root certificate of a private ACME server. |
| `TLS_STORAGE` | `DATA_DIR/certs` | Directory for the ACME account key and certificates. |

`PUBLIC_URL` must be an `https://` URL with a host name, not an IP address, when `TLS_ENABLED` is true.

The server answers a request as the dashboard and API only when its `Host` header equals the host of `PUBLIC_URL`. A request to the same server by another name, such as its IP address, is treated as a request to an unknown site host. Browse to the address you put in `PUBLIC_URL`.

`TEST_DATABASE_URL` is read by the test suite only. The server ignores it.

## Instance settings

The root user reads and changes settings with `GET` and `PATCH /api/v1/admin/settings`, or in the dashboard under Admin. Other users get a 404. A `PATCH` sends only the keys to change inside a `settings` object. The server rejects unknown keys and invalid values and applies nothing when any key is invalid.

```sh
curl -X PATCH http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"retention.keepVersions":20}}'
```

Changes apply on the next request. No setting needs a restart.

### Retention and limits

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `retention.keepVersions` | 10 | 1 to 10000 | Finalized versions kept per site, counting the live one. |
| `limits.maxFilesPerVersion` | 2500 | 1 to 1000000 | Files in one version. |
| `limits.maxVersionBytes` | 1073741824 | 1 to 2^50 | Total bytes in one version. |
| `limits.maxFileBytes` | 268435456 | 1 to 2^50 | Bytes in one file. Also caps the upload body. |
| `limits.maxGateCodeMailsPerHour` | 100 | 0 to 1000000 | Sign-in code emails that gated sites may send together in one hour. Past the limit a code request gets the same answer and no email goes out. 0 turns the limit off. Per-site and per-address limits still apply. |
| `limits.maxPendingBytes` | 4294967296 | 0 to 2^50 | Bytes that all unfinished versions on the instance may declare together. A manifest that would pass the total gets 429 `too_many_pending`. 0 turns the limit off. A site can also hold at most 3 unfinished versions and a user 10. Unfinished versions expire after 24 hours. |

### Hosting

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `hosting.baseDomain` | host of `PUBLIC_URL` | A bare host name, or empty for the default | Domain under which subdomain mode serves `{slug}.{baseDomain}`. |
| `hosting.subdomainEnabled` | false | boolean | Serves sites at `{slug}.{baseDomain}`. Needs a passed wildcard check. |
| `hosting.pathEnabled` | true | boolean | Serves sites at `{PUBLIC_URL}/s/{slug}/`. |
| `hosting.customDomainsEnabled` | true | boolean | Serves verified custom domains. Switching it off leaves stored domains alone. |
| `hosting.maxDomainsPerSite` | 10 | 1 to 50 | Custom domains one site can hold. |
| `hosting.wildcardStatus` | `unchecked` | read-only | Result of the last wildcard check: `unchecked`, `ok` or `failed`. |
| `hosting.wildcardCheckedAt` | null | read-only | When the last check finished. |
| `hosting.wildcardDetail` | empty | read-only | Why the last check failed. |
| `hosting.wildcardFailures` | 0 | read-only | Consecutive failed startup or scheduled checks while subdomain mode was on. |
| `hosting.subdomainDisabledReason` | null | read-only | `wildcard_check_failed` when the server switched subdomain mode off after 3 failed checks. |

At least one of `hosting.subdomainEnabled` and `hosting.pathEnabled` must stay on. See [Hosting modes](hosting-modes.md).

### Access and people

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `sites.defaultAccessMode` | `public` | `public`, `restricted` | Access mode of sites created from now on. `password` is not allowed because a password belongs to one site. |
| `invites.whoCanInvite` | `root` | `root`, `workspaceAdmins` | Who may create invites. |
| `workspaces.whoCanCreate` | `any` | `any`, `root` | Who may create shared workspaces. |

### Email

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `email.provider` | `none` | `none`, `smtp`, `resend` | Mail sender. |
| `email.fromAddress` | empty | A plain email address | Sender address. Both providers need it. |
| `email.fromName` | empty | Up to 100 characters | Sender name. |
| `email.smtp.host` | empty | Host name or IP address | SMTP server. |
| `email.smtp.port` | 587 | 1 to 65535 | SMTP port. |
| `email.smtp.security` | `starttls` | `tls`, `starttls`, `none` | `tls` is implicit TLS, usually port 465. `none` sends mail without encryption, and the server then refuses to log in with a user name and password unless the host is `localhost` or a loopback address. |
| `email.smtp.username` | empty | Up to 255 characters | Leave empty for a server without authentication. |
| `email.smtp.password` | write-only | Up to 1024 characters | Reads return `email.smtp.passwordSet`. An empty string clears it. |
| `email.resend.apiKey` | write-only | Up to 1024 characters | Reads return `email.resend.apiKeySet`. An empty string clears it. |

See [Email, invites and password resets](email-and-users.md).

### Certificates

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `tls.dnsProvider` | `none` | `none`, `cloudflare` | DNS provider for the wildcard certificate. Used only with built-in HTTPS. |
| `tls.cloudflareApiToken` | write-only | Up to 1024 characters | Cloudflare API token. Reads return `tls.cloudflareApiTokenSet`. |

### Analytics

| Key | Default | Allowed values | Meaning |
|---|---|---|---|
| `analytics.enabledByDefault` | true | boolean | Whether sites created from now on count page views. |
| `analytics.retentionDays` | 395 | 0, or 30 to 3650 | Days of rolled-up analytics to keep. 0 keeps everything. |
| `analytics.rawDays` | 31 | 14 to 365 | Days of individual events to keep once they are rolled up. |
| `analytics.maxmindAccountId` | empty | Digits, up to 20 | MaxMind account ID. |
| `analytics.maxmindLicenseKey` | write-only | Up to 1024 characters | MaxMind licence key. Reads return `analytics.maxmindLicenseKeySet`. |
| `analytics.geoStatus` | `unconfigured` | read-only | Result of the last GeoLite2 download: `unconfigured`, `ok` or `failed`. |
| `analytics.geoUpdatedAt` | null | read-only | When a database was last installed. |
| `analytics.geoCheckedAt` | null | read-only | When the last download attempt ran. |
| `analytics.geoDetail` | empty | read-only | Why the last attempt failed, or a note about the last download. |

See [Analytics](analytics.md).

### Write-only settings

The SMTP password, the Resend API key, the Cloudflare API token and the MaxMind licence key are write-only. The server encrypts each with AES-256-GCM before it stores it and never returns it. A read shows only whether one is set. See [Security](security.md#secrets-at-rest).
