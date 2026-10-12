<div align="center">

<img src="assets/icon.svg" alt="" width="88" />

# OpenHere

**Self-hosted static site hosting for AI agents. An agent publishes a built folder over the API and gets a link back.**

[![License](https://img.shields.io/github/license/zachlagden/openhere?style=flat-square)](LICENSE)
[![Stars](https://img.shields.io/github/stars/zachlagden/openhere?style=flat-square)](https://github.com/zachlagden/openhere/stargazers)
[![Last Commit](https://img.shields.io/github/last-commit/zachlagden/openhere?style=flat-square)](https://github.com/zachlagden/openhere/commits/main)
[![Issues](https://img.shields.io/github/issues/zachlagden/openhere?style=flat-square)](https://github.com/zachlagden/openhere/issues)
[![CI](https://img.shields.io/github/actions/workflow/status/zachlagden/openhere/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/zachlagden/openhere/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/zachlagden/openhere?style=flat-square)](https://github.com/zachlagden/openhere/releases/latest)

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-6-3178C6?style=flat-square&logo=typescript&logoColor=white)
![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-4-06B6D4?style=flat-square&logo=tailwindcss&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-default-003B57?style=flat-square&logo=sqlite&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-optional-4169E1?style=flat-square&logo=postgresql&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-image-2496ED?style=flat-square&logo=docker&logoColor=white)

[Install](#install) · [Agents](#agents) · [How it works](#how-it-works) · [Troubleshooting](#troubleshooting) · [Report Bug](https://github.com/zachlagden/openhere/issues) · [Request Feature](https://github.com/zachlagden/openhere/issues)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/sites-dark.png" />
  <img src="docs/screenshots/sites-light.png" alt="The OpenHere dashboard listing six demo sites in two folders, with their addresses, access modes and analytics switches" width="800" />
</picture>

</div>

---

## What is OpenHere?

You give OpenHere a built folder, such as a Next.js static export or a single HTML file. It serves that folder at a link, keeps a history of versions, and lets you restore an old one. An AI agent publishes by calling a REST API, and the dashboard covers everything else: sites, folders, access, domains, analytics, members and server settings. The dashboard has no upload button by design, so publishing is API-only.

It is built for one person or a small team on a VPS. It has no public sign-up, no billing and no public profiles. The first user becomes the root user through a setup flow, and everyone else joins by invite. It is one Go binary with the dashboard embedded, and it stores data in SQLite or PostgreSQL.

OpenHere is an independent open source project. It is not affiliated with, endorsed by or connected to [here.now](https://here.now) or its operator. It is a self-hosted alternative built from public documentation, and it shares no code with here.now.

---

## Features

| Feature | Description |
| --- | --- |
| Publishing and versions | Three calls: describe the files, upload each one, finalize. The server checks sizes and SHA-256 hashes, makes the version live in one step, keeps a version history, restores an older version and rejects a publish that is based on a stale version. |
| Hosting modes | Serves a site at `{slug}.example.com` (subdomain), at `/s/{slug}/` on the server address (path, sandboxed) or at a custom domain. Single-page app mode, `404.html`, range requests and ETags are supported. |
| Access modes | Each site is public, protected by a password, or restricted to instance users plus email and domain allowlists. Allowlisted visitors get an 8-digit code by email. |
| Custom domains | Add a domain to a site, prove ownership with a DNS TXT record, and mark one verified domain as the primary address. |
| Built-in HTTPS | The server can obtain its own ACME certificates, including on-demand ones for verified custom domains and a Cloudflare DNS-01 wildcard certificate for subdomain mode. Or run it behind Traefik, Coolify or Caddy. |
| Analytics | Counts page views and unique visitors without cookies or scripts, keeps daily rollups, and can add country and city from a MaxMind GeoLite2 database. |
| Workspaces and roles | Workspaces with `admin`, `write` and `read` roles, grants for a whole workspace, one folder or one site, flat folders, and a personal workspace for every user. |
| Invites and accounts | Invites by link or email, password resets, browser sessions and API keys. The root user can disable accounts. |
| Email sign-in for agents | An agent asks for a code by email, you give it the code, and the agent receives an API key. When the instance cannot send email, the agent asks for a dashboard key instead. |
| Agent skill | `skills/openhere` teaches Claude Code, Codex, Cursor and other agents to sign in, publish a folder and update the same site later. |
| Dashboard | Screens for sites, versions, access, domains, analytics, workspaces, members, invites, API keys and instance administration, in light and dark. |
| API with OpenAPI | Every dashboard action has a REST route. The binary serves an OpenAPI 3.1 description and an agent guide to signed-in users. |
| Security hardening | Argon2 password hashing with a concurrency limit, per-address and per-email rate limits, sealed secrets, a path-mode sandbox, and no unauthenticated route except hosted sites and the sign-in and gate pages. See [Security](docs/security.md). |

---

## Screenshots

<table>
  <tr>
    <td width="50%"><img src="docs/screenshots/site-analytics.png" alt="The analytics tab of a site, with totals, a views by day chart and unique visitors" /><br />Analytics for one site</td>
    <td width="50%"><img src="docs/screenshots/site-access.png" alt="The access tab of a site, with public, password and restricted modes" /><br />Access modes for one site</td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/screenshots/admin-policy.png" alt="The instance admin screen for who can invite, create workspaces and the default access mode" /><br />Instance admin settings</td>
    <td width="50%"><img src="docs/screenshots/gate-page.png" alt="The password page a visitor sees on a protected site" /><br />The gate page of a protected site</td>
  </tr>
</table>

All screenshots use fictional demo data.

---

## Install

Pick one route: [Docker](#with-docker), a [release binary](#from-a-release-binary) or a [build from source](#build-from-source). Whichever you choose, `PUBLIC_URL` must match the address you type in the browser.

### With Docker

The release workflow publishes a multi-architecture image (linux/amd64 and linux/arm64) to GitHub Container Registry, tagged with the exact version, the minor version and `latest`.

```sh
docker run -d --name openhere \
  -p 8080:8080 \
  -v openhere-data:/data \
  ghcr.io/zachlagden/openhere:latest
docker logs openhere
```

While the instance has no users, the log shows a one-time setup token:

```json
{"level":"INFO","msg":"setup required: send this token with the first setup request","setup_path":"/api/v1/setup","setup_token":"..."}
```

Open `http://localhost:8080`, enter the token, an email address and a password of 12 to 128 characters. That user becomes the root user. The token is discarded after setup.

**The dashboard answers only on the host in `PUBLIC_URL`.** It defaults to `http://localhost:8080`. A request with any other `Host`, such as `http://127.0.0.1:8080` or the machine's IP address, is treated as a request to an unknown site and returns 404. On a server you reach another way, set `PUBLIC_URL` to the address you use and match the published port:

```sh
docker run -d --name openhere \
  -p 8080:8080 \
  -v openhere-data:/data \
  -e PUBLIC_URL=http://203.0.113.10:8080 \
  ghcr.io/zachlagden/openhere:latest
```

PostgreSQL instead of SQLite, with a private network between the two containers:

```sh
docker network create openhere-net

docker run -d --name openhere-db --network openhere-net \
  -e POSTGRES_USER=openhere -e POSTGRES_PASSWORD=change-me -e POSTGRES_DB=openhere \
  -v openhere-pg:/var/lib/postgresql/data \
  postgres:17-alpine

docker run -d --name openhere --network openhere-net \
  -p 8080:8080 \
  -v openhere-data:/data \
  -e DATABASE_URL=postgres://openhere:change-me@openhere-db:5432/openhere \
  ghcr.io/zachlagden/openhere:latest
```

Replace `change-me` with a real password. The data volume still holds the published files and `secret.key`, so keep it.

The project ships a `Dockerfile` and no Docker Compose file, by design. A compose file would be a second description of how to run the server that could drift from the first. Coolify builds from the `Dockerfile` directly. To build the image yourself, run `docker build -t openhere .` in a clone and use `openhere` in place of the registry name. [Install with Docker](docs/install-docker.md) covers bind mounts, built-in HTTPS and upgrades.

### From a release binary

Each release attaches archives for linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64, plus a `SHA256SUMS` file. The release workflow builds them from the tag. Replace `0.1.1` with the version you want, and `linux_amd64` with your platform.

```sh
mkdir openhere-download && cd openhere-download
curl -fLO https://github.com/zachlagden/openhere/releases/download/v0.1.1/openhere_0.1.1_linux_amd64.tar.gz
curl -fLO https://github.com/zachlagden/openhere/releases/download/v0.1.1/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS
tar -xzf openhere_0.1.1_linux_amd64.tar.gz
./openhere_0.1.1_linux_amd64/openhere
```

On macOS, compare the output of `shasum -a 256 <archive>` with its line in `SHA256SUMS`. The binary has no flags. Configuration comes from environment variables. With none set, it listens on `:8080`, expects `http://localhost:8080`, stores everything in `./data` and uses SQLite. The first start logs the setup token as above. Stop it with Ctrl+C.

[Install from a binary](docs/install-binary.md) has a systemd unit, a service user, an environment file and backups.

### Build from source

You need the Go version in `go.mod` (1.26.9), Node 22 and pnpm. `corepack enable` provides the pnpm version that `web/package.json` pins. The server embeds the dashboard at build time, so build the dashboard first.

```sh
git clone https://github.com/zachlagden/openhere.git
cd openhere
corepack enable
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=dev" -o openhere ./cmd/openhere
./openhere
```

`go run ./cmd/openhere` works too. A binary built without the dashboard still starts, logs a warning and answers dashboard routes with a 503.

### Reverse proxy and Cloudflare

Behind Traefik, Coolify or Caddy, leave `TLS_ENABLED` unset, keep the server on a loopback or private address and set these variables. [Configuration](docs/configuration.md) lists every variable and instance setting.

| Variable | Value |
| --- | --- |
| `PUBLIC_URL` | The address people use, for example `https://sites.example.com`. |
| `TRUSTED_PROXIES` | The address or range the proxy connects from. The server reads `X-Forwarded-For` and `X-Forwarded-Proto` only from these peers. |
| `CLIENT_IP_HEADER` | Optional. Behind Cloudflare, `CF-Connecting-IP`, so rate limits and analytics see the visitor and not a Cloudflare address. |

The proxy must pass the original `Host` header. [Reverse proxy](docs/reverse-proxy.md) has Traefik, Coolify and Caddy examples and the firewall rule that makes `CLIENT_IP_HEADER` safe. [Built-in HTTPS](docs/built-in-https.md) covers the server getting its own certificates.

### First run

1. Open `PUBLIC_URL`, enter the setup token from the logs, and create the root user.
2. Open Settings, then API keys, and create a key. The dashboard shows it once.
3. Install the agent skill on the machine where your agent runs, as described under [Agents](#agents).
4. Ask the agent to publish a built folder. It prints the site address. A new site is public until you change its access mode.

### Uninstall

For Docker, run `docker rm -f openhere`. Add `docker volume rm openhere-data` to delete the sites, the SQLite database and `secret.key` with it. For a binary, stop the service, delete the binary and delete the data directory. Back up first if you may want the sites again, as [Upgrading and backups](docs/upgrading-and-backups.md) describes.

---

## Agents

Install the skill for the agents on your machine:

```sh
npx skills add zachlagden/openhere --skill openhere -g
```

The dashboard's Copy agent setup prompt button gives your agent this command, your instance URL and the sign-in route that fits your instance, which is an email code when the instance can send email and an API key from Settings otherwise. An agent that cannot install skills reads the agent guide and calls the API itself. The skill needs `curl`, `jq` and `sha256sum` (or `shasum`). [`skills/README.md`](skills/README.md) covers manual installs.

The skill saves the credentials to `~/.openhere/credentials` with mode 0600. Its scripts sign in, publish and list sites:

```sh
printf '%s' "$KEY" | bash ~/.claude/skills/openhere/scripts/auth.sh save-key --instance http://localhost:8080
bash ~/.claude/skills/openhere/scripts/publish.sh dist
```

```
created site quiet-meadow-k3p7: 2 file(s), 1118 bytes
url=http://localhost:8080/s/quiet-meadow-k3p7/
slug=quiet-meadow-k3p7
version=3dc773d32a644d4592e6d59be5862057
```

The script keeps the site in `.openhere/state.json` of the project. Run it again from the same project and it updates the same site and notices changes made from elsewhere. It refuses files that look like secrets.

Without the skill, any client can use the REST API. This example publishes the files in `./dist` and needs `curl`, `jq` and file names without spaces:

```sh
URL=http://localhost:8080
KEY=sk_...
cd dist

files=$(find . -type f | sed 's|^\./||' | while read -r f; do
  jq -n --arg p "$f" --argjson s "$(stat -c %s "$f")" --arg h "$(sha256sum "$f" | cut -d' ' -f1)" \
    '{path:$p,size:$s,sha256:$h}'
done | jq -s .)

resp=$(curl -fsS -X POST "$URL/api/v1/publish" \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d "{\"displayName\":\"Launch page\",\"files\":$files}")

echo "$resp" | jq -r '.uploads[] | "\(.path) \(.url)"' | while read -r path upload; do
  curl -fsS -X PUT "$URL$upload" -H "Authorization: Bearer $KEY" --data-binary @"$path" -o /dev/null
done

curl -fsS -X POST "$URL$(echo "$resp" | jq -r .finalizeUrl)" -H "Authorization: Bearer $KEY" | jq -r .url
```

The last command prints the site address. Nothing goes live until the finalize call succeeds. To publish a new version of an existing site, send `PUT /api/v1/publish/{slug}` with the full file list and the `baseVersionId` of the live version. [Agents](docs/agents.md), the [agent guide](docs/agent-guide.md) and the [API reference](docs/openapi.yaml) describe sign-in, errors and every route.

---

## Privacy and analytics

The server counts page views from the requests it serves. For each view it stores the site, the time, the request path without its query string, the status class, the host of the referring page, the browser family, the operating system family and the device type parsed from the `User-Agent` header, and a visitor hash. If you configure a MaxMind GeoLite2 database, it also stores the country and city that match the visitor's IP address.

It never stores the IP address, the raw `User-Agent`, a query string, a cookie or any other header value. The visitor hash is an HMAC-SHA256 of the client address, the `User-Agent`, the site and the UTC date under a secret key, so a visitor gets a different hash each day and on each site. The values it stores are coarse and derived from the request, but a country and city come from the IP address, so decide whether they fit your legal duties. Leave GeoLite2 unconfigured to store no location. See [Analytics](docs/analytics.md).

---

## How it works

```
request
  │  Host header
  ▼
hosting.Router
  ├── host of PUBLIC_URL        API, dashboard, and /s/{slug}/ in path mode
  ├── {slug}.{baseDomain}       site server, subdomain mode
  ├── verified custom domain    site server
  └── anything else             404
```

One Go process serves everything, and site hosts never reach API authentication. The API is plain `net/http` handlers over services in `internal/`, so business logic tests without HTTP. Raw SQL with `$1` placeholders runs on SQLite and PostgreSQL, and goose applies embedded migrations on start.

### Publishing

1. `POST /api/v1/publish` lists every file with its path, size and SHA-256 and creates a pending version. The response has an upload URL per file and a finalize URL.
2. Each file goes up with `PUT` as raw bytes. The server checks size and hash and stores nothing on a mismatch.
3. `POST` to the finalize URL. One transaction moves the site's live-version pointer, so a version is never half live.

Pending versions expire after 24 hours, and a site holds at most 3 of them. Hourly maintenance, the wildcard check, analytics rollups and the GeoLite2 refresh run in the same process. The server keeps rate limits and upload locks in memory, so run one process per data directory. Horizontal scaling is unsupported. The data directory holds the SQLite file, the files of each version under `sites/`, `secret.key`, and the GeoLite2 database and certificates when you use them.

---

## Documentation

| Guide | Contents |
| --- | --- |
| [Install with Docker](docs/install-docker.md), [from a binary](docs/install-binary.md) | `docker run` for SQLite and PostgreSQL, release archives, systemd |
| [Configuration](docs/configuration.md) | Every environment variable and instance setting |
| [Hosting modes](docs/hosting-modes.md), [reverse proxy](docs/reverse-proxy.md), [built-in HTTPS](docs/built-in-https.md) | Subdomain, path and custom domains, caching, Traefik, Coolify, Caddy, Cloudflare, ACME |
| [Security](docs/security.md), [access control](docs/access-control.md) | Threat model, public routes, rate limits, gated sites, viewer cookies |
| [Publishing](docs/publishing.md), [agents](docs/agents.md), [agent guide](docs/agent-guide.md), [agent skill](skills/README.md) | Roles, grants, versions, path rules, credentials, sign-in |
| [Email and users](docs/email-and-users.md), [analytics](docs/analytics.md), [dashboard](docs/dashboard.md) | SMTP and Resend, invites, what is recorded, MaxMind, screens |
| [Upgrading and backups](docs/upgrading-and-backups.md) | What is on disk, backup, restore, upgrades |
| [API reference](docs/openapi.yaml) | OpenAPI 3.1 description of every route |

---

## Tech stack

| Layer | Technology |
| --- | --- |
| Server | Go 1.26.9, standard library `net/http`, a single static binary with `CGO_ENABLED=0` |
| Database | SQLite through `modernc.org/sqlite` (no CGO), or PostgreSQL through `pgx`, with raw SQL and goose migrations |
| Certificates | certmagic, with `libdns` for the Cloudflare DNS challenge |
| Analytics | `maxminddb-golang` for GeoLite2 lookups |
| Dashboard | React 19, TypeScript 6, Vite 8, Tailwind CSS 4, shadcn/ui components, react-router 8, TanStack Query 5 |
| Tooling | pnpm 10 and Node 22 for the dashboard, Playwright end-to-end tests |
| Operations | Structured JSON logs through `log/slog`, optional Sentry SDK reporting |
| Packaging | A multi-stage `Dockerfile` onto distroless, and a GitHub Actions release workflow |

---

## Project structure

```
cmd/openhere/        entry point: config, wiring, servers, background tasks
internal/
├── api/             HTTP handlers and middleware, kept thin
├── auth/            passwords, sessions, API keys, setup
├── workspaces/      workspaces, grants and the one authorization function
├── sites/, publish/ folders and sites, manifests, versions, finalize, restore
├── hosting/         host router, file serving, path-mode sandbox, viewer gate
├── access/          access modes, viewer cookies, email codes
├── domains/, certs/ custom domain verification, ACME certificates
├── analytics/       view recording, rollups, reports, GeoLite2
├── mail/, store/    SMTP and Resend senders, database access and migrations
└── ...              settings, invites, ratelimit, proxy, secrets, sitefiles, sitepath
web/                 the dashboard, built into internal/ui/bundle
skills/openhere/     the agent skill
docs/                guides, the OpenAPI description and screenshots
```

---

## Development

Build the dashboard before the server, because the server embeds its output. CI runs these checks:

```sh
gofmt -l . && go vet ./... && golangci-lint run && go test -race ./...
pnpm --dir web lint && pnpm --dir web typecheck && pnpm --dir web test && pnpm --dir web build
pnpm --dir web e2e
```

`gofmt -l .` must print nothing. The Go tests run on SQLite, and on PostgreSQL as well when `TEST_DATABASE_URL` points at a throwaway database. The end-to-end run drives a built server in Chrome through setup, publishing and every screen. See [CONTRIBUTING.md](CONTRIBUTING.md) for the pull request process and the tests on both databases. Report security problems privately, as [SECURITY.md](SECURITY.md) describes.

### Releases

Pushing an annotated tag named `vX.Y.Z` runs `.github/workflows/release.yml`. It checks the tag format, builds the four archives with `SHA256SUMS`, pushes the multi-architecture image to `ghcr.io/zachlagden/openhere`, and publishes a GitHub release with the notes from the matching `## [X.Y.Z]` section of `CHANGELOG.md`. A tag with a suffix, such as `v0.2.0-rc.1`, is a prerelease and gets no `latest` or minor-version image tag. See the [changelog](CHANGELOG.md) for what changed in each version.

---

## Troubleshooting

<details>
<summary>The dashboard shows 404 on 127.0.0.1 or the machine's IP address</summary>

The dashboard answers only when the `Host` header equals the host of `PUBLIC_URL`. Any other host is treated as an unknown site. Open the address in `PUBLIC_URL`, or set `PUBLIC_URL` to the address you use and make its port match the published port.
</details>
<details>
<summary>The setup token is not in the logs</summary>

The server logs the token only while the database has no users. After setup it logs none, and a restart logs none. For a fresh start, look for the `setup required` line right after startup. If a user exists, sign in.
</details>
<details>
<summary>Sign-in returns 429</summary>

Login allows 30 attempts per address and 10 failed attempts per address and email pair in 15 minutes, and a limited request carries `retry_after`. Wait that long. The counters live in memory, so a restart clears them. See the rate limit table in [Security](docs/security.md#rate-limits).
</details>
<details>
<summary>Behind Cloudflare, every visitor looks like one address</summary>

A proxy such as Traefik replaces `X-Forwarded-For` with the Cloudflare edge address. Set `TRUSTED_PROXIES` to the proxy's range and `CLIENT_IP_HEADER=CF-Connecting-IP`, and restrict the origin to Cloudflare's address ranges. [Reverse proxy](docs/reverse-proxy.md#cloudflare-in-front-of-traefik-or-coolify) explains why.
</details>
<details>
<summary>Subdomain mode cannot be enabled</summary>

The server refuses with `wildcard_not_verified` until a wildcard check passes. The check resolves a random name under the base domain and asks the server to answer on it, so it needs wildcard DNS, a matching certificate and a proxy route. Fix those, then run Check now under Admin, Hosting. On `PUBLIC_URL=http://localhost:8080` the check fails unless your resolver maps `*.localhost` to loopback, and path mode still works. See [Hosting modes](docs/hosting-modes.md#wildcard-check).
</details>
<details>
<summary>A password site shows "HTTPS required"</summary>

A gated site in path mode needs a `Secure` cookie, which browsers refuse over plain HTTP. Use HTTPS, or use subdomain or custom domain mode for gated sites. See [Access control](docs/access-control.md#viewer-cookies).
</details>
<details>
<summary>A republished CSS or JavaScript file still looks old behind a CDN</summary>

Site files send `Cache-Control: no-cache` and revalidate by `ETag`. Only files with a content hash in their name under `_next`, `assets`, `static` or `build` send a one-year `immutable` header. A CDN cache rule or a browser cache setting can keep a file longer than the server allows, so purge the file and check the rules for that host. See [Caching](docs/hosting-modes.md#caching) and [Reverse proxy](docs/reverse-proxy.md#cloudflare-and-site-files).
</details>
<details>
<summary>A custom domain stays pending</summary>

The domain needs a TXT record at `_openhere-challenge.<your domain>` with the token the dashboard shows. DNS can take a while to spread, so press Check now again. An unverified claim is released after 7 days. A verified domain is re-checked once a day, and 3 consecutive misses send it back to pending. See [Custom domains](docs/hosting-modes.md#custom-domains).
</details>
<details>
<summary>MaxMind rejects a new licence key</summary>

A new key can take a few minutes to become active, and an early check fails with "MaxMind rejected the account ID or licence key". Wait, then press Check now under Admin, Analytics. See [Analytics](docs/analytics.md#country-and-city-lookups-with-maxmind).
</details>
<details>
<summary>The server refuses published files</summary>

A manifest path must be relative and use `/`. The server rejects empty and absolute paths, `.` and `..` segments, empty segments, backslashes, control characters, paths over 1024 bytes, and two paths that differ only by case, with 400 `invalid_path`. The agent skill also stops with exit code 6 when a file looks like a secret, such as a private key or a `.pem` file. Remove the file from the folder. See [Workspaces, sites and publishing](docs/publishing.md).
</details>
<details>
<summary>Sign-in or a site password check returns 503 server_busy</summary>

The server hashes at most `ARGON2_CONCURRENCY` passwords at once (default 2, each using 64 MiB) and a request that waits more than 5 seconds gets 503 with `retry_after`. Retry after that time, or raise the setting on a server with spare memory.
</details>
<details>
<summary>The server stops with "unable to open database file" in Docker</summary>

The image runs as the user `nonroot` (uid 65532), and the data directory must be writable by it. A named volume works. For a bind mount, run `sudo chown 65532:65532 /srv/openhere/data` first. The server creates `DATA_DIR` with mode 0700, so a backup job that runs as another user needs its own access.
</details>

---

## Star history

<a href="https://github.com/zachlagden/openhere/stargazers">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://repo-star-history.zachlagden.uk/svg?repos=zachlagden/openhere&type=Date&theme=dark&legend=top-left&format=png" />
   <source media="(prefers-color-scheme: light)" srcset="https://repo-star-history.zachlagden.uk/svg?repos=zachlagden/openhere&type=Date&legend=top-left&format=png" />
   <img alt="Star History Chart for zachlagden/openhere" src="https://repo-star-history.zachlagden.uk/svg?repos=zachlagden/openhere&type=Date&legend=top-left&format=png" />
 </picture>
</a>

---

## Support

If this project is useful to you, consider supporting development:

<a href="https://github.com/sponsors/zachlagden">
  <img src="https://img.shields.io/badge/Sponsor_on_GitHub-ea4aaa?style=for-the-badge&logo=github&logoColor=white" alt="Sponsor on GitHub" />
</a>

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

<div align="center">

**[Report Bug](https://github.com/zachlagden/openhere/issues)** · **[Request Feature](https://github.com/zachlagden/openhere/issues)**

Made by [Zach Lagden](https://github.com/zachlagden)

</div>
