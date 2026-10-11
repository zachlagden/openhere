---
name: openhere
description: Publishes built static sites, pages, reports, dashboards and folders to the user's own self-hosted OpenHere instance and returns a shareable link, then updates, restores and protects them. Use whenever the user wants to publish, host, deploy, share, put online or update a static site, HTML page, report, dashboard, export or build output folder on OpenHere or on their own self-hosted instance, including "put this online", "give me a link to this", "share this page" and "update the site I published", or when ~/.openhere/credentials exists. Handles sign-in with an emailed code or a dashboard API key. Static files only. Do not use it for other hosts (Vercel, Netlify, GitHub Pages, here.now, Cloudflare Pages), or for servers, databases and long-running processes.
---

# OpenHere

OpenHere is a self-hosted service that serves folders of already built static files at a link. You publish through its REST API with the scripts in this skill. Run them by their full path inside this skill's directory, for example `bash <skill-dir>/scripts/publish.sh dist`.

Needs `curl`, `jq` and `sha256sum` (or `shasum`). A script that lacks one stops and names it.

## When to use it

Use it when the user wants a static result online: a built site, an HTML report, an exported dashboard, a folder of documents or images. The instance serves files only. It cannot run server code, a database or a background process. If the project needs those, say so and do not publish.

Publish only what the user asked for. A site can be public, so check with the user before publishing private material.

## 1. Find the instance and sign in

Run `bash <skill-dir>/scripts/auth.sh check`. It reads, in this order:

1. `OPENHERE_URL` and `OPENHERE_API_KEY` from the environment.
2. `~/.openhere/credentials`, a JSON file with `url` and `key`.

If it prints `credentials work`, go to step 2. If it says there is no instance URL, ask the user for their OpenHere address (for example `https://sites.example.com`). If it says the key was rejected, sign in again.

Sign in in one of two ways. Pass `--instance <url>` to each command.

**Email code**, when the instance can send email:

1. Ask the user for the email address of their OpenHere account.
2. Run `auth.sh request-code <email> --instance <url>`.
3. Tell the user a code was emailed, and ask for the 8-digit code. It works once, for 10 minutes.
4. Run `auth.sh verify-code <email> <code> "<your agent name>" --instance <url>`. This saves the key.

**Dashboard key**, when `request-code` exits with code 7 (`email_not_configured`):

1. Tell the user to open the dashboard, go to Settings, API keys, create a key and paste it to you.
2. Run `printf '%s' "<key>" | bash <skill-dir>/scripts/auth.sh save-key --instance <url>`.

Both routes write `~/.openhere/credentials` with mode 0600. Never print, repeat or commit the key, and never put it in a file in the project.

## 2. Build

Publish the output folder, never the source. Find it in this order: `out/` (Next.js static export), `dist/`, `build/`, `public/`, or the folder the user names. If none exists, build first with the project's own command (`pnpm build`, `npm run build`) and look again. A Next.js app needs `output: "export"` in its config to produce static files. Stop and tell the user if the app needs a server.

`index.html` must sit at the top of the folder you publish. A single file is published by putting it alone in a folder.

The script already skips `.git`, `node_modules`, hidden files (except `.well-known`), lock files, symlinks and source maps. It refuses with exit code 6 when a file looks like a secret (private keys, access tokens, `.pem` and `.key` files). Never work around this. Tell the user which files it named and let them remove the files. Use `--allow-secrets` only after the user says to publish them.

## 3. Publish

```bash
bash <skill-dir>/scripts/publish.sh dist
```

Useful options: `--slug NAME` (pick the site name), `--name "Title"`, `--spa` (client-routed single page apps), `--folder NAME` and `--workspace SLUG` (when creating), `--dry-run` (list files and stop), `--jobs N`. Run `publish.sh --help` for all of them.

The script prints three lines on stdout and a summary on stderr:

```
url=https://sites.example.com/s/quiet-meadow-k3p7/
slug=quiet-meadow-k3p7
version=4f0c...
```

It remembers the site in `.openhere/state.json` in the current directory and adds `.openhere/` to `.gitignore` when the project has one. The file holds the slug, the instance URL and the live version id, and no key. A second `publish.sh dist` from the same project updates the same site and sends the stored version id as `baseVersionId`, so it notices changes made from elsewhere. Run the script from the project root so it finds the file.

Always give the user the `url` on a line of its own. Say whether you created or updated the site. If the output has a `notice:` line, relay it, because it explains an address that is switched off or unreachable.

## Exit codes

| Code | Meaning | Do this |
|---|---|---|
| 0 | Published | Share the `url`. |
| 1 | Usage error | Read the message. A plain `http://` instance that is not localhost is refused. Use `https`. |
| 2 | Missing tool, no credentials, or key rejected | Install the tool, or sign in again (step 1). |
| 3 | Version conflict | The live site changed since the last publish from here. Tell the user and show the live version id. Rerun with `--force` only if the user agrees to replace it. |
| 4 | The instance refused the request | The message names the code. See `references/api.md`. |
| 5 | Network failure, or the instance kept failing | The script already retried with backoff. Check the address and try later. |
| 6 | Nothing to publish, or files look like secrets | Build first, or ask the user about the named files. |
| 7 | `auth.sh` only: instance cannot send email | Use the dashboard key route. |

The scripts retry transient failures (timeouts, 5xx, `rate_limited`, `server_busy`) up to 5 times with backoff and honour `retry_after`. `too_many_pending` is not retried: tell the user to wait or delete pending versions.

## Other tasks

- **List sites:** `bash <skill-dir>/scripts/status.sh` prints `slug url updated` per line. Filter with `--query` or `--workspace`.
- **Update a site:** change the files, rebuild, run `publish.sh` again from the same project, or pass `--slug`.
- **Restore an older version:** `GET /api/v1/publish/{slug}/versions`, then `POST /api/v1/publish/{slug}/versions/{versionId}/restore`. Details are in `references/api.md`.
- **Change who can see a site** (public, password, restricted), **single page app mode**, or a **custom domain**: read `references/access-and-domains.md`. Ask the user first, and tell them what to do for DNS records, which you cannot create.
- **Delete a site:** only when the user asks for exactly that. `DELETE /api/v1/sites/{slug}`.

## What to tell the user

- The final `url`, always, alone on its line.
- When you need them: the email code, a dashboard key, a DNS record, or a decision on a conflict or a secret-looking file.
- What you skipped (the script lists skipped file counts), if it matters.
- Never the key, never a site password except the one you just set for them at their request.
- Plainly, when something failed: the exit code, the message, and the next step.

If the script cannot be run (no shell), call the API directly. `references/api.md` has the three-call publish flow and every error code.
