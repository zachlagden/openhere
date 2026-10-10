# Agents

The server is built for AI agents to publish and manage sites. Everything the dashboard can do, an agent can do through the REST API. The dashboard cannot upload a site, so publishing is only possible through the API.

Two documents describe the API to an agent:

- [`openapi.yaml`](openapi.yaml), the OpenAPI 3.1 reference.
- [`agent-guide.md`](agent-guide.md), a short guide written for an agent to read. The binary embeds both, and signed-in users and keys can download them at `GET /api/v1/docs/openapi.yaml` and `GET /api/v1/docs/agent-guide.md`. Neither is public.

This page explains the pieces from the operator's side.

## Giving an agent access

An agent needs an instance URL and an API key. It looks for them in `~/.selfhost/credentials`, a JSON file:

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

Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. An agent should branch on `code`, not `message`. `retry_after` appears on 429 responses.

| Status | Meaning |
|---|---|
| 400 | `validation_failed`, `invalid_json` or `invalid_path` |
| 401 | `unauthenticated`: the key is missing or revoked |
| 403 | `forbidden`: the user can see the resource but their role does not allow the action |
| 404 | `not_found`: the resource does not exist or the user cannot see it |
| 409 | A conflict such as `slug_taken`, `version_conflict`, `missing_files` or `email_not_configured` |
| 413 | A limit was exceeded: `too_many_files`, `file_too_large`, `version_too_large` or `body_too_large` |
| 429 | `rate_limited`: wait `retry_after` seconds |

## Keeping the agent guide accurate

`docs/agent-guide.md` is embedded in the binary, so a change to the API that affects agents needs an edit to that file in the same commit.
