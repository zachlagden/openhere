# Renaming the project

The name `selfhost` is a placeholder. This checklist lists every place it appears and the steps to rename safely. In the steps, `NEWNAME` is the new name, `newname` its lower-case form and `github.com/OWNER/NEWNAME` the new module path.

Find what is left after each step with:

```sh
grep -rIn 'selfhost' . --exclude-dir=node_modules --exclude-dir=.git --exclude=pnpm-lock.yaml
```

## Places that carry the name

The first column says whether a user, an agent or an operator can see the name outside the source tree. Those rows need a migration note in the changelog.

| Visible | Where | What it holds |
|---|---|---|
| No | `go.mod`, and every import of `github.com/zachlagden/selfhost/...` in `cmd/` and `internal/` | The Go module path |
| No | `cmd/selfhost/` (directory) | The main package. Rename the directory and update `go build ./cmd/selfhost` everywhere. |
| No | `cmd/selfhost/main.go` | The prefix of the startup error message (`selfhost:`) |
| Operator | `Dockerfile` | The binary name in `/out/selfhost`, `/selfhost` and `ENTRYPOINT ["/selfhost"]` |
| Operator | `.github/workflows/release.yml` | The archive names `selfhost_${VERSION}_${os}_${arch}` and the binary inside them |
| Operator | `.github/workflows/ci.yml` | The image tag `selfhost:ci` and the database name `selfhost_test` |
| Operator | GitHub Container Registry image | Derived from the repository name by the release workflow. Renaming the repository changes the image name. |
| Operator | `.gitignore` | `/selfhost` and `/cmd/selfhost/selfhost` |
| Operator | `.env.example`, `CLAUDE.md`, `CONTRIBUTING.md` | Example database names `selfhost` and `selfhost_test` |
| Operator | `README.md`, every file in `docs/`, `SECURITY.md`, `CHANGELOG.md` | The title, `docker build -t selfhost`, `docker run --name selfhost`, the `selfhost-data` volume and `selfhost-db` container, `/usr/local/bin/selfhost`, `/var/lib/selfhost`, `/etc/selfhost`, the systemd unit and user, and prose |
| Operator | `internal/store/sqlite.go` (`sqliteFileName`) | The SQLite file `DATA_DIR/selfhost.db` |
| User | `internal/config/agent.go` (`AgentCredentialsDir`) | The agent credentials directory `~/.selfhost/credentials`. It also appears in `docs/agent-guide.md`, `docs/openapi.yaml`, `docs/agents.md`, `web/src/test/fixtures.ts` and the generated `web/src/api/schema.d.ts`. |
| User | `internal/domains/name.go` (`ChallengeLabel`) | The DNS challenge label `_selfhost-challenge`. Users have created TXT records at this name. |
| User | `internal/hosting/probes.go` (`ProbePath`) | The wildcard probe path `/.well-known/selfhost-check/` |
| User | `internal/hosting/gate_paths.go` (`gateBase`, `CallbackPath`) | The gate paths `/.well-known/selfhost-gate/` and `/.well-known/selfhost-auth` |
| User | `docs/openapi.yaml` | The title `selfhost API`, the descriptions that mention the paths above, and the examples |
| User | `web/src/brand.ts` (`BRAND_NAME`), `web/index.html` | The dashboard name and the page title |
| No | `web/src/api/client.ts` (`REQUESTED_WITH`) | The value of the `X-Requested-With` header. The server checks only that it is not empty. |
| No | `internal/secrets/box.go` | The string `selfhost-derive|` that scopes the HMAC key for agent sign-in codes |
| No | `web/e2e/run.mjs`, `web/e2e/server.mjs`, `web/e2e/flow.mjs`, other e2e files | The temporary binary name, `SELFHOST_BIN` and temporary directory prefixes |
| No | Test files | Fixtures and expectations that use the names above |

Cookie names carry no project name. They are `session`, `__Host-session`, `viewer`, `__Host-viewer` and `__Secure-viewer`. Do not rename them as part of this change, because that signs every user and viewer out.

## Steps

1. Create a branch `chore/rename-project`.
2. Change the module path with `go mod edit -module github.com/OWNER/NEWNAME`, then replace the old import prefix in every `.go` file, for example with `grep -rl 'github.com/zachlagden/selfhost' --include='*.go' . | xargs sed -i 's|github.com/zachlagden/selfhost|github.com/OWNER/NEWNAME|g'`. Run `gofmt -l .`, `go build ./...` and `go vet ./...`.
3. Rename `cmd/selfhost` with `git mv cmd/selfhost cmd/newname`. Update the build commands in `Dockerfile`, `.github/workflows/release.yml`, `.gitignore`, `web/e2e/run.mjs`, `CLAUDE.md`, `README.md`, `CONTRIBUTING.md` and the install guides.
4. Change the binary and image names in the `Dockerfile` and the archive names in the release workflow. Keep the release workflow free of comments.
5. Change `web/src/brand.ts`, `web/index.html` and the `REQUESTED_WITH` value. Update the dashboard tests that expect the old title.
6. Decide each user-visible name with the table below, change the constants, then update `docs/openapi.yaml` and regenerate the dashboard types with `pnpm --dir web gen:api`.
7. Replace the name in the README and in every file in `docs/`, in `CONTRIBUTING.md`, in `.github/` templates and in `CLAUDE.md`. Keep the name in as few places as the project rules allow.
8. Rename the repository on GitHub, then update `<repository url>` placeholders and any links. GitHub redirects the old repository URL. The container image path follows the new repository name.
9. Add a changelog entry under Unreleased that names every migration in the next section.
10. Run every check in [CONTRIBUTING.md](../CONTRIBUTING.md#check-your-work), including both databases and `docker build`. Search for the old name again with the `grep` command above. Only deliberate compatibility code should remain.

## User-visible names and what to do about them

| Name | Effect of changing it | Safe approach |
|---|---|---|
| Agent credentials directory `~/.selfhost` | Agents that saved a key there no longer find it and ask the user again. The dashboard's copy prompt and `GET /api/v1/instance` report the new path. | Change the constant. In the agent guide, tell agents to read the new path first and fall back to the old one. Note it in the changelog. |
| DNS challenge label `_selfhost-challenge` | The daily re-check of verified custom domains looks for the new label, finds none, and after 3 consecutive daily misses sends the domain back to `pending_verification` and stops serving it. | Make the verifier accept both labels for at least one release, show the new label to new claims, and tell operators to add the new TXT record. Then drop the old label. |
| Wildcard probe path `/.well-known/selfhost-check/` | The probe is created and consumed by the same process, so a rename takes effect with no data change. | Rename the constant. |
| Gate paths `/.well-known/selfhost-gate/` and `/.well-known/selfhost-auth` | Sign-in links and open gate forms that use the old path fail. Codes and tokens are short-lived. | Rename the constants and note that viewers may need to reload the gate page. Update the allowlist of unauthenticated routes in `docs/security.md`, `CLAUDE.md` and the route test. |
| SQLite file `selfhost.db` | A new name makes the server create an empty database next to the old one. | Keep the old name, or rename the file during upgrade and document it. Never change the constant without a rename step. |
| Binary and image name | Service units, scripts and `docker run` commands that use the old name break. | Publish the new image name for the first release under the new name, note it in the changelog and keep the old image name unchanged. |
| `selfhost-derive|` scope string | The key that hashes agent sign-in codes changes, so codes issued before the upgrade stop working. | Change it in a release with no other migration, or accept that agent codes in flight (10 minutes) expire. |
| Release archive names | Download scripts that match the old name break. | Note it in the changelog. |
