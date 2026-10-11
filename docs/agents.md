# Agents

The server is built for AI agents to publish and manage sites. Everything the dashboard can do, an agent can do through the REST API. The dashboard cannot upload a site, so publishing is only possible through the API.

Two documents describe the API to an agent:

- [`openapi.yaml`](openapi.yaml), the OpenAPI 3.1 reference.
- [`agent-guide.md`](agent-guide.md), a short guide written for an agent to read. The binary embeds both, and signed-in users and keys can download them at `GET /api/v1/docs/openapi.yaml` and `GET /api/v1/docs/agent-guide.md`. Neither is public.

This page explains the pieces from the operator's side.

## Giving an agent access

An agent needs an instance URL and an API key. It looks for them in `~/.openhere/credentials`, a JSON file:

```json
{"url": "https://sites.example.com", "key": "sk_..."}
```

An agent creates the directory with mode 0700 and the file with mode 0600, reads the file at the start of a session before it asks for anything, and sends the key as `Authorization: Bearer <key>`. The directory name comes from the constant `AgentCredentialsDir`, and `GET /api/v1/instance` and the `verify-code` response report the path.

An agent without credentials has two ways to get a key.

### Email code

This works when the instance can send email (see [Email, invites and password resets](email-and-users.md)).

1. The agent sends `POST /api/v1/auth/agent/request-code` with `{"email": "alex@example.com"}`. The response is the same for every address. The server emails an 8-digit code to an existing, enabled account only.
2. You read the code from the email and give it to the agent.
3. The agent sends `POST /api/v1/auth/agent/verify-code` with `{"email": "...", "code": "12345678", "label": "my agent"}` and receives an API key once. The label defaults to `agent`.

A code works once, expires after 10 minutes and allows 5 attempts. Requests are limited to 3 per hour per email address, 10 per 15 minutes per IP address and 100 per hour for the instance. The 50 active keys per user limit applies. The code lets the agent create a key with the full access of your account, so give it only to an agent you started.

When the instance cannot send email, `request-code` returns 409 `email_not_configured`.

### API key

Create a key in the dashboard under Settings, API keys. The dashboard shows the full key once. Give it to the agent, which saves it to the credentials file. The dashboard also has a button that copies a ready-made setup prompt for an agent, filled in with your instance URL.

A key has the full access of its user, as every request that uses it is made as that user. Revoke a key in the dashboard or with `DELETE /api/v1/me/keys/{id}`. The server stores only a hash of each key.

## Publishing

Publishing takes three calls, and nothing goes live until the last one succeeds.

1. `POST /api/v1/publish` creates a site and a pending version. The body lists every file with its `path`, `size`, `sha256` and, optionally, `contentType`. The response has one upload URL per file and a `finalizeUrl`. `PUT /api/v1/publish/{slug}` does the same for an existing site and accepts `baseVersionId` for conflict detection.
2. `PUT` each file's raw bytes to its upload URL, with the same credentials. The server checks the size and SHA-256 and stores nothing on a mismatch.
3. `POST` the `finalizeUrl`. The version becomes live in one step. A missing file returns 409 `missing_files` with the paths to upload again.

See [Workspaces, sites and publishing](publishing.md) for limits, versions and path rules.

## Access modes

A new site follows `sites.defaultAccessMode`, which is `public` unless the root user changes it. An agent changes a site with `PATCH /api/v1/publish/{slug}/access`.

| Mode | Who can open the site |
|---|---|
| `public` | Anyone with the link |
| `password` | Anyone with the site's password (8 to 128 characters) |
| `restricted` | Instance users with read access, plus the email addresses and domains on the allowlists |

Tell agents to ask before they publish something private. See [Access control](access-control.md).

## Errors

Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. An agent should branch on `code`, not `message`. `retry_after` appears on `rate_limited` and `server_busy` responses, and `too_many_pending` has none because waiting a fixed time does not clear it.

| Status | `code` | What to do |
|---|---|---|
| 400 | `validation_failed`, `invalid_json` | Fix the request body and send it again. |
| 400 | `invalid_path` | A manifest path is unsafe or malformed. Use relative paths with `/` separators and no `..`, and send a corrected manifest. |
| 400 | `size_mismatch`, `hash_mismatch` | The uploaded bytes differ from the manifest. Recompute the size and SHA-256 from the file on disk, then upload that file again. |
| 401 | `unauthenticated` | The key is missing or revoked. Ask the user for access again. |
| 403 | `forbidden` | The user can see the resource but their role does not allow the action. Tell the user. |
| 404 | `not_found` | The resource does not exist or the user cannot see it. |
| 404 | `file_not_in_manifest` | You uploaded a path the version does not list. Upload only paths from the `uploads` array. |
| 409 | `version_conflict` | The live version changed since your `baseVersionId`. Read `currentVersionId`, list the live files, rebuild the manifest and publish again. |
| 409 | `missing_files` | Finalize found missing or damaged files. Upload the paths in `missing`, then finalize again. |
| 409 | `version_finalized` | The version is already live and cannot take uploads. Start a new publish. |
| 409 | `version_live` | You tried to delete the live version. Restore another version first. |
| 409 | `version_not_finalized` | You tried to restore a version that never went live. Pick a finalized version. |
| 409 | `slug_taken` | Choose another slug. |
| 409 | `domain_not_verified` | A domain must pass its DNS check before it can become primary. Verify it first. |
| 409 | `email_not_configured` | The instance cannot send email. Use an API key. |
| 410 | `version_expired` | The pending version passed its 24 hours. Start a new publish. |
| 413 | `too_many_files`, `file_too_large`, `version_too_large`, `body_too_large` | A limit was exceeded. Send fewer or smaller files, or ask the user to raise the limit. |
| 415 | `unsupported_media_type` | JSON endpoints need `Content-Type: application/json`. File uploads send raw bytes. |
| 429 | `rate_limited` | Wait `retry_after` seconds, then try again. |
| 429 | `too_many_pending` | Too many unfinished versions. The response has no `retry_after`. Finalize or delete your pending versions, or wait for them to expire after 24 hours. |
| 503 | `server_busy` | The server is hashing passwords. Wait `retry_after` seconds, then try again. |

Calls in the sections above can return further codes, such as `folder_has_grants`. Treat an unknown code as a failure and show its `message` to the user.

## Keeping the agent guide accurate

`docs/agent-guide.md` is embedded in the binary, so a change to the API that affects agents needs an edit to that file in the same commit.
