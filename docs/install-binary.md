# Install from a binary

The server is one static Go binary with the dashboard embedded. It needs no runtime beyond a data directory, and optionally a PostgreSQL database.

## Get a binary

Tagged releases attach archives for linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64, with a `SHA256SUMS` file. No release exists yet, so build from source until one does. After a release, verify a download with `sha256sum -c SHA256SUMS`.

## Build from source

You need:

- the Go version named in `go.mod`.
- Node 22 and pnpm, to build the dashboard. The `web/package.json` file pins the pnpm version, and `corepack enable` provides it.

The server embeds the dashboard at build time, so build the dashboard first:

```sh
git clone https://github.com/zachlagden/openhere
cd openhere
corepack enable
pnpm --dir web install --frozen-lockfile
pnpm --dir web build
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=1.2.3" -o openhere ./cmd/openhere
```

`-X main.version` sets the version that `GET /api/v1/instance` reports. It defaults to `dev`. The dashboard build writes to `internal/ui/bundle`, which is not committed. A binary built without it still runs, logs a warning and answers dashboard routes with a 503.

The binary has no command line flags. All configuration comes from environment variables (see [Configuration](configuration.md)).

## Try it

```sh
./openhere
```

With no configuration the server listens on `:8080`, expects to be reached at `http://localhost:8080`, stores everything under `./data` and uses SQLite. It prints a JSON log line with a `setup_token` once while the database has no users. Open `http://localhost:8080`, enter the token, an email address and a password. Stop the server with Ctrl+C. It shuts down gracefully, waiting up to 15 seconds for requests and flushing buffered analytics.

## Run as a service

Create a user and a data directory, then install the binary:

```sh
sudo useradd --system --home-dir /var/lib/openhere --shell /usr/sbin/nologin openhere
sudo install -d -o openhere -g openhere -m 0750 /var/lib/openhere
sudo install -m 0755 openhere /usr/local/bin/openhere
```

Put the environment in a file that only root and the service can read, `/etc/openhere/openhere.env`:

```sh
DATA_DIR=/var/lib/openhere
LISTEN_ADDR=127.0.0.1:8080
PUBLIC_URL=https://sites.example.com
TRUSTED_PROXIES=127.0.0.1
```

Create the directory with `sudo install -d -m 0700 /etc/openhere` and keep the file at mode `0600`, owned by root. systemd reads `EnvironmentFile` as root before it drops to the service user, so the service user needs no access to it.

A systemd unit, `/etc/systemd/system/openhere.service`:

```ini
[Unit]
Description=OpenHere static site hosting
After=network-online.target
Wants=network-online.target

[Service]
User=openhere
Group=openhere
EnvironmentFile=/etc/openhere/openhere.env
ExecStart=/usr/local/bin/openhere
Restart=on-failure
TimeoutStopSec=30
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/openhere

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now openhere
journalctl -u openhere | grep setup_token
```

For built-in HTTPS on ports 80 and 443 as a non-root user, add `AmbientCapabilities=CAP_NET_BIND_SERVICE` to the `[Service]` section, or run `setcap 'cap_net_bind_service=+ep' /usr/local/bin/openhere`. Then set the `TLS_*` and `ACME_*` variables. See [Built-in HTTPS](built-in-https.md). Behind a proxy, keep `LISTEN_ADDR` on the loopback address and see [Reverse proxy](reverse-proxy.md).

## The data directory

`DATA_DIR` holds everything the server writes, apart from PostgreSQL. [Upgrading and backups](upgrading-and-backups.md) lists the contents. Keep it on a persistent disk, owned by the service user, and run only one server process on it.

## Backups and upgrades

Stop the service, copy the data directory (and dump PostgreSQL if you use it), and start it again. To upgrade, replace the binary and restart. The server applies database migrations on start. Take a backup first, because a migration cannot be undone by installing the old binary. See [Upgrading and backups](upgrading-and-backups.md).
