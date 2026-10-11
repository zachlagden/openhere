# selfhost

A self-hosted, open source alternative to [here.now](https://here.now): hosting for already built static sites, aimed at AI agents, with a dashboard and a REST API. It is built for one person or a small team running it on a VPS.

"selfhost" is a placeholder name and will change. See [docs/rename-checklist.md](docs/rename-checklist.md).

The project has not had a release yet. It is feature complete for its first version and is being prepared for a public release. See the [changelog](CHANGELOG.md).

## What it is and is not

You give it a built folder, such as a Next.js static export or a single HTML file. It serves that folder at an address of your choosing and keeps a history of versions. Agents publish and manage sites through the REST API, and the dashboard covers everything else.

It is not a public sign-up service. It has no billing and no public profiles. The first user is created through a setup flow and becomes the root user. Everyone else joins by invite. The dashboard has no way to upload a site, because agents do that through the API.

## Features

- **Publishing.** Three calls: describe the files, upload each one, finalize. Size and SHA-256 checks, atomic go-live, version history, restore and conflict detection.
- **Hosting.** Subdomains (`{slug}.example.com`), paths (`/s/{slug}/`, sandboxed) and custom domains verified with a DNS TXT record. Single-page app mode, `404.html`, range requests and ETags.
- **Access control.** Each site is public, password protected, or restricted to instance users plus email and domain allowlists.
- **Teams.** Workspaces with `admin`, `write` and `read` roles, grants scoped to a workspace, a folder or a site, flat folders, and invites by link or email.
- **Agents.** An OpenAPI 3.1 description and an agent guide, both embedded and served to signed-in users. An agent gets a key through an email code or an API key you create.
- **Analytics.** Page views and unique visitors without cookies or scripts, with optional country and city lookups.
- **HTTPS.** Run behind Traefik or Caddy, or let the server obtain certificates itself, including a wildcard certificate through Cloudflare DNS.
- **Operations.** One static binary with the dashboard embedded. SQLite by default and PostgreSQL when you set `DATABASE_URL`. JSON logs, optional Sentry or GlitchTip error reporting.

## Quick start with Docker

```sh
docker build -t selfhost .
docker run -d --name selfhost -p 8080:8080 -v selfhost-data:/data selfhost
docker logs selfhost
```

The log shows a one-time setup token while the instance has no users:

```json
{"level":"INFO","msg":"setup required: send this token with the first setup request","setup_path":"/api/v1/setup","setup_token":"..."}
```

Open `http://localhost:8080`, enter the token, an email address and a password of 12 to 128 characters. That user becomes the root user. Reach the server by the host in `PUBLIC_URL`, which defaults to `http://localhost:8080`. On a server you reach by another name, set `PUBLIC_URL`. A request with any other `Host` header is treated as a request to an unknown site and returns 404.

A new site is public: anyone with its address can read it until you set another access mode. See [Access control](docs/access-control.md).

The project ships no Docker Compose file. See [Install with Docker](docs/install-docker.md) for PostgreSQL, bind mounts and built-in HTTPS.

## Run from source

You need the Go version in `go.mod`, and Node 22 with pnpm to build the dashboard.

```sh
corepack enable
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
go run ./cmd/openhere
```

See [Install from a binary](docs/install-binary.md) for a release build, a systemd unit and backups.

## How an agent publishes

An agent reads `~/.selfhost/credentials`, a JSON file with the instance URL and an API key, and sends the key as a bearer token. Create a key in the dashboard under Settings, API keys, or let the agent ask for an email code. See [Agents](docs/agents.md).

This example publishes the files in `./dist`. It needs `curl` and `jq`, and file names without spaces.

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

The last command prints the site address. Nothing goes live until the finalize call succeeds. To publish a new version of an existing site, send `PUT /api/v1/publish/{slug}` with the full file list and the `baseVersionId` of the live version.

## Architecture

One Go process serves everything. `hosting.Router` looks at the `Host` header: the host of `PUBLIC_URL` goes to the API and dashboard (and to `/s/` for path mode), `{slug}.{baseDomain}` and verified custom domains go to the site server, and any other host gets a 404. Site hosts never reach API authentication.

- The API is plain `net/http` handlers over services in `internal/`. Business logic lives in those packages, so it tests without HTTP.
- Raw SQL with `$1` placeholders runs on SQLite (`modernc.org/sqlite`, no CGO) and on PostgreSQL (`pgx`). Goose applies embedded migrations on start.
- Published files live on disk under `DATA_DIR/sites/<siteId>/versions/<versionId>/files/`. A site's live version is a pointer in the database, which one transaction moves.
- The dashboard in `web/` (Vite, React, TypeScript, Tailwind CSS) is built into `internal/ui/bundle` and embedded.
- Background tasks run in the same process: hourly maintenance, the wildcard check, analytics rollups and the GeoLite2 refresh.

The server keeps rate limits and upload locks in process memory, so run one process per data directory. Horizontal scaling is unsupported.

## Privacy and analytics

The server counts page views from the requests it serves. For each view it stores the site, the time, the request path without its query string, the status class, the host of the referring page, the browser family, the operating system family and the device type parsed from the `User-Agent` header, and a visitor hash. If you configure a MaxMind GeoLite2 database, it also stores the country and city that match the visitor's IP address.

It never stores the IP address, the raw `User-Agent`, a query string, a cookie or any other header value. The visitor hash is an HMAC-SHA256 of the client address, the `User-Agent`, the site and the UTC date under a secret key, so a visitor gets a different hash each day and on each site. The values it stores are coarse and derived from the request, but a country and city come from the IP address, so decide whether they fit your legal duties. Leave GeoLite2 unconfigured to store no location. See [Analytics](docs/analytics.md).

## Documentation

| Guide | Contents |
|---|---|
| [Install with Docker](docs/install-docker.md) | `docker run` for SQLite and PostgreSQL, volumes, ports |
| [Install from a binary](docs/install-binary.md) | Build from source, systemd, data directory |
| [Configuration](docs/configuration.md) | Every environment variable and instance setting |
| [Hosting modes](docs/hosting-modes.md) | Subdomain, path and custom domains, the wildcard check, base domains |
| [Reverse proxy](docs/reverse-proxy.md) | Traefik, Coolify and Caddy, trusted proxies |
| [Built-in HTTPS](docs/built-in-https.md) | ACME certificates, wildcard certificates through Cloudflare |
| [Security](docs/security.md) | Threat model, public routes, secrets at rest, known limits |
| [Access control](docs/access-control.md) | Public, password and restricted sites, viewer cookies |
| [Workspaces, sites and publishing](docs/publishing.md) | Roles, grants, versions, path rules |
| [Agents](docs/agents.md) and the [agent guide](docs/agent-guide.md) | Credentials, sign-in, publishing |
| [Email, invites and password resets](docs/email-and-users.md) | SMTP and Resend, invites, resets |
| [Analytics](docs/analytics.md) | What is recorded, MaxMind setup, retention |
| [Dashboard](docs/dashboard.md) | Screens and how it is served |
| [Upgrading and backups](docs/upgrading-and-backups.md) | Disk layout, backup, restore, upgrades |
| [API reference](docs/openapi.yaml) | OpenAPI 3.1 description of every route |

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for the build, the tests on both databases and the commit rules. Report security problems privately, as [SECURITY.md](SECURITY.md) describes.

## Licence

MIT. See [LICENSE](LICENSE).
