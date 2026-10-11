# Agent guide

This guide tells an AI agent how to publish and manage static sites on this instance. A site is a folder of already built files. The instance serves it at an address it returns to you.

If your environment can install skills, prefer the OpenHere skill: `npx skills add zachlagden/openhere --skill openhere -g`. Its scripts do the steps below for you. Use this guide when you cannot install it.

All paths below are relative to the instance URL and start with `/api/v1`. Bodies are JSON unless a step says otherwise. The full reference is `GET /api/v1/docs/openapi.yaml`.

## Authenticate

Read `~/.openhere/credentials` first. It is a JSON file with the instance URL and an API key:

```json
{"url": "https://sites.example.com", "key": "sk_..."}
```

If the file exists, use its `url` and send the key with every request as `Authorization: Bearer <key>`. If a request returns 401, the key was revoked, so ask the user for access again.

If the file does not exist, get a key in one of two ways.

### Email code

Use this when the instance can send email.

1. `POST /api/v1/auth/agent/request-code` with `{"email": "<the user's email>"}`. The response is the same for every address.
2. Ask the user for the 8-digit code that arrives in their inbox. A code lasts 10 minutes and works once.
3. `POST /api/v1/auth/agent/verify-code` with `{"email": "...", "code": "12345678", "label": "<your agent name>"}`. The response holds the key once.
4. Save `{"url": "<instance URL>", "key": "<key>"}` to `~/.openhere/credentials`. Create the directory with mode 0700 and the file with mode 0600.

If `request-code` returns 409 `email_not_configured`, the instance cannot send email. Use an API key instead.

### API key

Ask the user to create a key in the dashboard under Settings, API keys, and to paste it to you. Save it to `~/.openhere/credentials` as shown above.

A key has the full access of its user. Never print it, log it or commit it.

## Publish a folder

Publishing takes three calls. Nothing goes live until the last one succeeds.

### 1. Describe the files

Walk the built folder. For every file, work out its path relative to the folder (use `/` separators), its size in bytes, its SHA-256 as 64 hex characters and, if you know it, its content type.

For a new site:

```
POST /api/v1/publish
{
  "displayName": "Launch page",
  "spaMode": false,
  "files": [
    {"path": "index.html", "size": 1204, "sha256": "<64 hex characters>", "contentType": "text/html; charset=utf-8"},
    {"path": "assets/app.js", "size": 53211, "sha256": "<64 hex characters>"}
  ]
}
```

All body fields except `files` are optional:

- `slug` picks the site name. The server generates one such as `quiet-meadow-k3p7` when you leave it out. A slug uses lowercase letters, digits and hyphens, and cannot change later.
- `workspace` is a workspace slug or id. Without it the server uses the `X-Workspace` header, then the user's personal workspace.
- `folder` is a folder id or name in that workspace.
- `description` is a short text of up to 1000 characters. `displayName` has at most 100.
- `spaMode` serves `index.html` for unknown paths that have no file extension. Turn it on for single page apps.

To publish a new version of an existing site, send `PUT /api/v1/publish/{slug}` with `files` and, to detect conflicts, `"baseVersionId": "<the live version id>"`. List every file of the new version, not only the changed ones.

The response holds `slug`, `versionId`, `url`, an `uploads` array and a `finalizeUrl`.

### 2. Upload every file

For each entry in `uploads`, send `PUT <url>` with the raw file bytes as the body. Use the same `Authorization` header and add the headers the entry lists. The server checks the size and hash against the manifest and stores nothing on a mismatch. The upload `url` is a path on the instance, so prefix it with the instance URL.

### 3. Finalize

`POST <finalizeUrl>` makes the version live. If any file is missing or damaged, the response is 409 `missing_files` and `missing` lists the paths. Upload those files again and call finalize again. Calling finalize on a finalized version returns the same result.

The response includes the site `url` and a `hosting` object. Tell the user the `url`. If `hosting.reachable` is false, read `hosting.notices` and tell the user what they say.

A pending version that is not finalized within 24 hours expires.

## Manage sites

| Task | Call |
|---|---|
| List sites you can reach | `GET /api/v1/sites?q=<text>&workspace=<slug>&folder=<id or none>&limit=50&cursor=<nextCursor>` |
| Read one site | `GET /api/v1/sites/{slug}` |
| Rename, move or change SPA mode | `PATCH /api/v1/sites/{slug}` with `displayName`, `description`, `folderId` (null for unfiled), `spaMode`, `analyticsEnabled` |
| Delete a site | `DELETE /api/v1/sites/{slug}` |
| List versions | `GET /api/v1/publish/{slug}/versions` |
| Make an older version live | `POST /api/v1/publish/{slug}/versions/{versionId}/restore` |
| Delete a version | `DELETE /api/v1/publish/{slug}/versions/{versionId}` |
| List live files | `GET /api/v1/sites/{slug}/files` |

## Workspaces and folders

Every user has a personal workspace. Shared workspaces have members with the roles `admin`, `write` and `read`.

| Task | Call |
|---|---|
| List workspaces | `GET /api/v1/workspaces` |
| Create a shared workspace | `POST /api/v1/workspaces` with `{"name": "Acme Ltd"}` |
| List folders | `GET /api/v1/workspaces/{workspace}/folders` |
| Create a folder | `POST /api/v1/workspaces/{workspace}/folders` with `{"name": "Reports"}` |
| Rename a folder | `PATCH /api/v1/workspaces/{workspace}/folders/{folder}` with `{"name": "..."}` |
| Delete a folder | `DELETE /api/v1/workspaces/{workspace}/folders/{folder}`, workspace admin only (its sites become unfiled). It fails with 409 `folder_has_grants` while grants limited to the folder exist. Add `?force=true` to delete those too |

Folders are flat and belong to one workspace. Names are unique per workspace regardless of case.

## Access modes

A new site is open to anyone with the link unless the instance default says otherwise. Change a site with `PATCH /api/v1/publish/{slug}/access`.

| Mode | Who can open the site |
|---|---|
| `public` | Anyone with the link |
| `password` | Anyone with the site's password. Send `{"mode": "password", "password": "<8 to 128 characters>"}` |
| `restricted` | Instance users with read access, plus the addresses and domains on the allowlists. Send `emailAllowlist` and `domainAllowlist` to replace those lists |

`GET /api/v1/publish/{slug}/access` shows the current settings. It never returns the password.

## Custom domains

A site can have its own domains. The server proves ownership with a DNS TXT record, and the user must create it at their DNS provider.

| Task | Call |
|---|---|
| Claim a domain | `POST /api/v1/sites/{slug}/domains` with `{"domain": "docs.example.org"}`. The response has a `challenge` (a TXT record name and value) and a `target` hint. |
| Check the record | `POST /api/v1/sites/{slug}/domains/{id}/verify`. The response says in plain language why a check failed. DNS changes take time, so repeat the call later. |
| Make it primary | `PATCH /api/v1/sites/{slug}/domains/{id}` with `{"isPrimary": true}` |
| List or remove | `GET /api/v1/sites/{slug}/domains`, `DELETE /api/v1/sites/{slug}/domains/{id}` |

Tell the user which record to create. Verification never looks at A or CNAME records, so the user also needs to point the domain at the instance.

## Analytics

`GET /api/v1/sites/{slug}/analytics?from=2026-01-01&to=2026-01-31` returns page views and unique visitors with ranked lists. Dates are inclusive and the range is at most 366 days. Switch counting on or off with `analyticsEnabled` on `PATCH /api/v1/sites/{slug}`.

## Instance facts

`GET /api/v1/instance` returns the public URL, the version, whether the instance can send email, which hosting modes are on and where agents keep credentials. Read it before you tell the user an address.

## Errors

Errors are JSON: `{"code": "...", "message": "...", "retry_after": 30}`. Branch on `code`, not on `message`.

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
