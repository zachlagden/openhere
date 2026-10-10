# Workspaces, sites and publishing

## Workspaces and sites

Every user has a personal workspace, created together with the account. It has one member, its owner, and cannot be deleted or shared. Any signed-in user can create shared workspaces with `POST /api/v1/workspaces` and becomes their admin.

A grant gives a user a role in a workspace. The roles are:

- `admin` manages members and grants, renames and deletes the workspace. Admin covers the whole workspace only.
- `write` creates, edits, moves and deletes the sites it covers. A workspace-wide `write` also creates and renames folders.
- `read` views and searches.

A grant covers the whole workspace, one folder (and the sites in it) or one site. A user can hold several scoped grants in one workspace, and the highest role among the grants that cover a resource applies. A user with only scoped grants sees only the covered folders and sites, and the workspace still appears in their list. A write grant on one folder lets the user create sites in that folder and nowhere else.

A resource the user cannot see returns 404. A resource the user can see but may not change returns 403. A workspace always keeps at least one workspace-wide admin: the last one cannot be removed or demoted. Add a member by the email of an existing account with `POST /api/v1/workspaces/{workspace}/members`. To bring in someone without an account, send an invite. See [Email, invites and password resets](email-and-users.md#invites).

Folders are flat and belong to one workspace. Names are unique per workspace regardless of case. Only a workspace-wide `admin` deletes a folder, and only a workspace-wide `write` or `admin` renames one. A grant limited to a folder never allows either, because deleting the folder removes every grant on it. Deleting a folder moves its sites to unfiled. The call fails with 409 `folder_has_grants` while grants limited to the folder exist. Remove them first, or add `?force=true` to delete the folder and those grants.

A site belongs to one workspace and at most one folder. Its slug is unique across the instance, uses 1 to 63 lowercase letters, digits and hyphens with no leading or trailing hyphen, and cannot be a reserved word such as `www`, `api` or `admin`. The server generates a slug such as `quiet-meadow-k3p7` when you give none. The slug cannot change. `POST /api/v1/workspaces/{workspace}/sites` creates an empty site. A workspace that contains sites cannot be deleted.

`GET /api/v1/sites` lists every site the caller can reach, newest first. Filter with `workspace`, `folder` (a folder id, or `none` for unfiled sites) and `q`, which matches the display name and slug without regard to case. Page with `limit` (default 50, at most 200) and the `cursor` from the previous response's `nextCursor`.

## Publishing

Publishing takes three steps, and a version serves nothing until the last one succeeds.

1. `POST /api/v1/publish` creates a site and a pending version. `PUT /api/v1/publish/{slug}` creates a pending version of an existing site. The body lists every file of the version with its `path`, `size`, `sha256` and `contentType`. The response holds one upload URL per file.
2. `PUT` each file's raw bytes to its upload URL. The server streams the body to a temporary file under `DATA_DIR/tmp`, checks the size and SHA-256 against the manifest, and moves the file to `DATA_DIR/sites/<siteId>/versions/<versionId>/files/<path>`. A mismatch stores nothing.
3. `POST /api/v1/publish/{slug}/versions/{versionId}/finalize` checks that every file exists with the right size and hash, checks the limits, and flips the site's live pointer in one transaction. A missing file returns 409 `missing_files` with the paths to upload again. Calling finalize again returns the same result.

Pass `baseVersionId` on `PUT /publish/{slug}` to detect conflicts. If the site's live version is not that id, the server returns 409 `version_conflict` with `currentVersionId`, and `GET /api/v1/sites/{slug}/files` lists the live files so an agent can reconcile. Finalize checks the base again inside its transaction, so two publishes built on the same base cannot both go live. Without `baseVersionId`, a publish overwrites whatever is live.

`GET /api/v1/publish/{slug}/versions` lists versions. `POST .../versions/{versionId}/restore` makes an older finalized version live without uploading, and `DELETE .../versions/{versionId}` removes one unless it is live. After each finalize the server deletes finalized versions beyond `retention.keepVersions`, never the live one. Pending versions that are not finalized within 24 hours expire, and an hourly maintenance task removes them, orphaned directories and stale upload files. Deleting a site removes its directory.

`GET /api/v1/sites/{slug}/files/{path}` returns a live file as `application/octet-stream` with `Content-Disposition: attachment`, so a stored HTML file never runs on the API origin.

Paths are relative with `/` separators. The server rejects empty and absolute paths, `.` and `..` segments, empty segments, backslashes, NUL, control and invisible formatting characters, paths over 1024 bytes and segments over 255 bytes. It also rejects two paths that differ only in letter case, and a path that is also the directory of another. Dotfiles such as `.well-known/` are allowed. The server never creates or follows symlinks.

Set a site's `spaMode` when creating it or with `PATCH /api/v1/sites/{slug}`. In SPA mode, a request that matches no file and whose last path segment has no file extension serves `index.html` with status 200.

Several processes must not share one `DATA_DIR`. The upload and finalize locks live in process memory.
