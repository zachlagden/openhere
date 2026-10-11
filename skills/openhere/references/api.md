# OpenHere API reference

## Contents

- Conventions
- Sign-in calls
- Publish flow
- Sites, versions and workspaces
- Errors and what to do
- Limits

## Conventions

Every path starts with `/api/v1` on the instance URL. Bodies are JSON, and file uploads send raw bytes. Send the key as `Authorization: Bearer <key>`. Branch on the `code` field of an error, never on `message`.

An error body looks like `{"code": "...", "message": "...", "retry_after": 30, "currentVersionId": "...", "missing": ["a.css"]}`. Only some codes carry the last three fields.

The scripts in `scripts/` wrap everything below. Call the API directly only for tasks the scripts do not cover. The full description is `GET /api/v1/docs/openapi.yaml` and `GET /api/v1/docs/agent-guide.md`, which need the key.

## Sign-in calls

Neither call needs a key.

| Call | Body | Result |
|---|---|---|
| `POST /auth/agent/request-code` | `{"email": "alex@example.com"}` | 200 with the same answer for every address. The instance emails an 8-digit code to an existing account. 409 `email_not_configured` when it cannot send email. |
| `POST /auth/agent/verify-code` | `{"email": "...", "code": "12345678", "label": "my agent"}` | 201 with `key`, shown once. A wrong, expired or reused code returns 400 `invalid_code`. |

A code works once, for 10 minutes and 5 attempts. Requests are limited to 3 per hour per address, so do not retry in a loop. A key has the full access of its user.

`GET /instance` (needs the key) returns `publicUrl`, `version`, `emailAvailable`, `hostingModes` and `agentCredentialsPath`.

## Publish flow

1. `POST /publish` creates a site and a pending version. `PUT /publish/{slug}` creates a pending version of an existing site.
2. `PUT <upload url>` once per file, with the raw bytes and the `Authorization` header.
3. `POST <finalizeUrl>` makes the version live.

Create body (`files` is the only required field):

```json
{
  "slug": "launch-page",
  "displayName": "Launch page",
  "description": "Spring launch",
  "folder": "Reports",
  "workspace": "acme",
  "spaMode": false,
  "files": [
    {"path": "index.html", "size": 1204, "sha256": "<64 hex characters>", "contentType": "text/html; charset=utf-8"}
  ]
}
```

Update body: `{"baseVersionId": "<live version id>", "files": [...]}`. List every file of the new version, not only the changed ones. Without `baseVersionId` the new version replaces whatever is live.

The response holds `slug`, `siteId`, `versionId`, `expiresAt`, `uploads` (`path`, `method`, `url`, `headers`), `finalizeUrl`, `url` and `hosting`. Upload URLs and the finalize URL are paths on the instance. Never send the key to a URL that is not on the instance.

Finalize returns `{"slug", "url", "hosting", "version": {"id", "live", ...}}`. Tell the user `url`. Read `hosting.notices` when `hosting.reachable` is false and relay them.

Paths are relative, use `/`, and may not be empty, absolute, contain `.` or `..` segments, backslashes or control characters, exceed 1024 bytes, or differ from another path only in letter case. A pending version expires after 24 hours.

## Sites, versions and workspaces

| Task | Call |
|---|---|
| List sites | `GET /sites?q=&workspace=&folder=&limit=50&cursor=` returns `sites` and `nextCursor` |
| Read a site | `GET /sites/{slug}` returns `{"site": {...}}` with `currentVersionId`, `url`, `accessMode`, `spaMode` |
| Rename or set SPA mode | `PATCH /sites/{slug}` with `displayName`, `description`, `spaMode`, `folderId` |
| Delete a site | `DELETE /sites/{slug}`. Ask the user first. |
| List versions | `GET /publish/{slug}/versions` (newest first, `live` marks the served one) |
| Restore a version | `POST /publish/{slug}/versions/{versionId}/restore` |
| Delete a version | `DELETE /publish/{slug}/versions/{versionId}` (not the live one) |
| List live files | `GET /sites/{slug}/files` |
| List workspaces | `GET /workspaces` |
| List or create folders | `GET` or `POST /workspaces/{workspace}/folders` with `{"name": "Reports"}` |

Restoring makes an older finalized version live without uploading anything. Old versions are pruned beyond the instance's retention setting, so a restore target can disappear.

## Errors and what to do

| Status | `code` | Do this |
|---|---|---|
| 400 | `validation_failed`, `invalid_json` | Fix the request body. |
| 400 | `invalid_path` | Rename the file. The message names the rule. |
| 400 | `size_mismatch`, `hash_mismatch` | The file changed during the upload. Rerun the script. |
| 400 | `invalid_code` | The email code is wrong or expired. Ask for a new code. |
| 401 | `unauthenticated` | The key is missing or revoked. Ask the user for a new key. |
| 403 | `forbidden` | The key's user cannot do this. Tell the user. |
| 404 | `not_found` | The site, folder or workspace does not exist or is not visible to the user. |
| 404 | `file_not_in_manifest` | Upload only the paths the response lists. |
| 409 | `version_conflict` | The live version is not the base you sent. `publish.sh` exits with 3 and prints it. Ask the user before using `--force`. |
| 409 | `missing_files` | `missing` lists paths to upload again. `publish.sh` does this itself. |
| 409 | `slug_taken` | Another site uses the slug. Pick another. |
| 409 | `version_finalized`, `version_expired`, `version_live`, `version_not_finalized` | Start a new publish, or pick another version. |
| 409 | `email_not_configured` | The instance cannot send email. Ask the user for a dashboard key. |
| 409 | `domain_not_verified` | Verify the domain's DNS record first. |
| 410 | `version_expired` | The pending version ran out of time. Publish again. |
| 413 | `too_many_files`, `file_too_large`, `version_too_large`, `body_too_large` | Send fewer or smaller files, or ask the user to raise the limit. |
| 415 | `unsupported_media_type` | JSON calls need `Content-Type: application/json`. |
| 429 | `rate_limited` | Wait `retry_after` seconds. The scripts do. |
| 429 | `too_many_pending` | Too many unfinished versions. There is no `retry_after`. Delete pending versions or wait up to 24 hours. |
| 503 | `server_busy` | Wait `retry_after` seconds. The scripts do. |

Treat any other code as a failure and show its `message` to the user.

## Limits

Set by the instance owner, with these defaults: 2500 files and 1 GiB per version, 256 MiB per file. A site can hold at most 3 unfinished versions and a user 10.
