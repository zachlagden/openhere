# Access modes, SPA mode and custom domains

Read this only when the user asks to protect a site, share it with certain people, or give it its own domain. Ask before you change who can see a site.

## Contents

- Access modes
- Change the access mode
- Single page apps
- Custom domains

## Access modes

| Mode | Who can open the site |
|---|---|
| `public` | Anyone with the link. This is the default unless the instance owner changed it. |
| `password` | Anyone with the site's password. |
| `restricted` | Users of the instance with read access, plus the email addresses and email domains on the allowlists. |

A password or restricted site shows a sign-in page to visitors. The site's address still answers, so a hard to guess slug matters for private material. Generated slugs are hard to guess.

## Change the access mode

`GET /api/v1/publish/{slug}/access` shows the settings and never returns the password.

`PATCH /api/v1/publish/{slug}/access` takes any of these fields:

```json
{"mode": "password", "password": "<8 to 128 characters>"}
{"mode": "restricted", "emailAllowlist": ["sam@example.com"], "domainAllowlist": ["example.org"]}
{"mode": "public"}
```

The allowlists replace the stored lists, so read them first and send the merged result. Each list holds at most 500 entries. Password mode needs a password to exist. Changing the mode, the password or a list signs every visitor out.

Allowlisted visitors who are not instance users sign in with an emailed code. That works only when the instance can send email (`emailCodeAvailable` in the access settings). Gated sites work best in subdomain mode. Over plain http in path mode (`/s/{slug}/`), gated sites cannot set their cookie.

Never put a site password in a file, a commit or a chat message the user did not ask for. Tell the user the password once, when you set it.

## Single page apps

Pass `--spa` to `scripts/publish.sh`, or send `{"spaMode": true}` to `PATCH /api/v1/sites/{slug}`. The site then serves `index.html` for paths that match no file and whose last segment has no file extension. Use it for client-routed apps. A static export with real HTML files per route does not need it.

## Custom domains

The user owns the DNS. The instance proves ownership with a TXT record.

1. `POST /api/v1/sites/{slug}/domains` with `{"domain": "docs.example.org"}`. The response has a `challenge` (`recordType`, `name`, `value`) and a `target` (where to point the domain).
2. Tell the user which two DNS records to create at their provider: the TXT record from `challenge`, and the CNAME from `target` (follow its `note` for apex domains or a CDN). You cannot create them.
3. After the user says the records exist, call `POST /api/v1/sites/{slug}/domains/{id}/verify`. If it fails, show the plain-language reason and try again later, because DNS changes take time.
4. To make it the main address, send `PATCH /api/v1/sites/{slug}/domains/{id}` with `{"isPrimary": true}`. A domain must be verified first (409 `domain_not_verified`).

`GET /api/v1/sites/{slug}/domains` lists domains and `DELETE /api/v1/sites/{slug}/domains/{id}` removes one. The instance limits domains per site (`maxDomainsPerSite` in `GET /api/v1/instance`) and may have custom domains turned off (`hostingModes.customDomains`).
