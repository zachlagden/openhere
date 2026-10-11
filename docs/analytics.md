# Analytics

The server counts page views itself, from the requests it serves. It injects no script into your sites, sets no cookie and reads nothing from the visitor's device.

## What is recorded

A view is an HTML document served to a `GET` request with status 200, or a custom `404.html` served with status 404. For each view the server stores:

- the site, the time (to the second) and the request path without its query string or fragment, at most 200 characters. The server counts a view only for a path that exists in the live version. A single-page app route that falls back to `index.html` counts as `/`, and a custom `404.html` counts under the path `/(not found)`, which appears in the status list and never in the page list. A request for a path that exists nowhere creates no view of its own. Each site stores at most 1000 distinct paths per day. Further paths count under `/(other)`. One visitor can add 120 views a minute, and bots 600 a minute per site. The server drops the rest.
- the status class, `2xx` or `4xx`.
- the host of the referring page, such as `news.example.org`, never the path or query. It is empty when the visitor came from the same host or sent no `Referer`.
- the browser family, the operating system family and the device type (`desktop`, `mobile`, `tablet`, `bot` or `other`), parsed from the `User-Agent` header.
- the country and city of the visitor's IP address, only when a GeoLite2 database is installed. Bots get none.
- a visitor hash, described below. Bots get none.

The server does not store the IP address, the raw `User-Agent` value, a query string, a cookie or any other header value. It uses the IP address in memory to look up the location and to compute the visitor hash, then drops it. The address is the client address after [trusted proxy](reverse-proxy.md) handling, so a forwarded address counts only when the connection comes from a `TRUSTED_PROXIES` address.

The browser, operating system, device type, referrer host, country and city are coarse values derived from the request. The country and city come from the visitor's IP address and are stored with the view, so a lookup turns a technical identifier into approximate location data. Whether that fits your legal duties, such as under the UK GDPR, is your decision. Leave GeoLite2 unconfigured to store no location.

The server does not count asset requests (scripts, styles, images, data), `HEAD` requests, partial (`Range`) and cached (304) responses, prefetches, redirects, gate pages and sign-in prompts, or any request while analytics is off for the site. A protected site records a view only after the gate has let the request through.

## How unique visitors work

The visitor hash is an HMAC-SHA256 over the client address, the `User-Agent`, the site id and the UTC date, truncated to 128 bits. For an IPv6 address the input is the /64 prefix. The key is a random 32-byte secret created on first use and stored encrypted in the database. No API returns it, and it does not change from day to day. The hash changes daily because the date is part of its input.

The same browser on the same site on the same day gets the same hash, so it counts once. The next day it gets a different hash, and so does another site. Someone without the key cannot reverse or link hashes across days. Someone who holds the database and the sealing key can recompute a hash for a known address, user agent, site and day, so treat a copy of the database together with the key as sensitive. Two consequences for the numbers:

- Unique counts for a range add up the daily counts, so a person who visits on three days counts three times.
- Two people behind one address with the same browser string count as one.

Visitors whose `User-Agent` looks like a crawler or an HTTP client, or is empty, count as `botViews` and as the `bot` device. They are left out of every other number. The bot list is a short built-in list of name fragments, so it catches common bots and not all of them.

## Turning it on and off

Each site has an `analyticsEnabled` switch (`PATCH /api/v1/sites/{slug}`). New sites take the `analytics.enabledByDefault` value. Switching a site off stops counting at once and keeps the data already recorded.

## Reading it

| Endpoint | Who may read it |
|---|---|
| `GET /api/v1/sites/{slug}/analytics` | Anyone with read access to the site. |
| `GET /api/v1/workspaces/{workspace}/analytics` | Users with a workspace-wide role. The report adds up the sites in the workspace that the user can see. It works for personal workspaces too. |
| `GET /api/v1/admin/analytics` | The root user. It covers every site and adds `droppedEvents`. |

Query parameters are `from` and `to` (RFC 3339 dates, both days inclusive, default the last 30 days, at most 366 days), `interval` (`day`, or `hour` for ranges of 14 days or less) and `top` (entries per ranked list, 1 to 50, default 10). The response holds totals, a time series, ranked lists for paths, referrers, countries, cities, browsers, operating systems, device types and status classes, `locationAvailable` and `generatedAt`. The dashboard shows the same data under each site, workspace and the admin analytics page.

## Performance

Serving never waits for analytics. The server puts each view in a bounded in-memory buffer (4096 events) and a background worker writes batches every 5 seconds or 200 events. When the buffer is full the server drops the event and adds one to `droppedEvents`, which the root report shows. The count restarts at 0 when the server starts. A failed database write drops only the affected events. A graceful shutdown writes what is still in the buffer. A crash loses at most a few seconds of events.

## Rollups and retention

Individual events stay for `analytics.rawDays` days (default 31). A maintenance task runs every hour and folds each finished UTC day into daily rollups per site: views and uniques in total and by path, referrer host, country, city, browser, operating system, device and status class, keeping the top 500 values of each list per day. Reports read the rollup for days that have one and raw events for the rest, so a range that spans both counts every view once. Running the task again changes nothing, and events that arrive late for a finished day are folded in at the next run.

The server deletes raw events older than `analytics.rawDays` once they are rolled up, and deletes everything older than `analytics.retentionDays` (default 395 days, 0 keeps all). Hourly charts need raw events, so `analytics.rawDays` has a minimum of 14. Deleting a site deletes its analytics.

## Country and city lookups with MaxMind

Location lookups use the free GeoLite2 City database from MaxMind. To switch them on:

1. Create a free GeoLite2 account at MaxMind and generate a licence key in the account portal.
2. Set `analytics.maxmindAccountId` (the numeric account ID) and `analytics.maxmindLicenseKey`, in the dashboard under Admin, Analytics or with `PATCH /api/v1/admin/settings`.

Within five minutes the server downloads `GeoLite2-City` from `download.maxmind.com` over HTTPS, checks its SHA-256 checksum when MaxMind provides one, and installs it as `DATA_DIR/geo/GeoLite2-City.mmdb` through an atomic rename, so a failed or partial download never replaces a working file. The lookup picks up a new file without a restart. `analytics.geoStatus`, `analytics.geoUpdatedAt`, `analytics.geoCheckedAt` and `analytics.geoDetail` show the state to the root user. The licence key is never logged or returned. Without credentials the location fields stay empty and reports show `locationAvailable: false`.

### When the server checks

The server looks at the database every 5 minutes and downloads only when one of these holds:

- No database file exists yet, or the file is older than 7 days. A file younger than 7 days is never downloaded again, whatever triggered the check.
- The last download failed, and either 6 hours have passed, the credentials differ from the ones that failed, or the server has restarted since and the failure is older than 5 minutes.
- You saved `analytics.maxmindAccountId` or `analytics.maxmindLicenseKey`, even with the same value. The save starts a check within a few seconds and skips the wait after a failure.
- You pressed Check now in the dashboard, or called `POST /api/v1/admin/analytics/geo/check`. This also skips the wait after a failure.

The server starts at most one check at a time. It starts at most one forced check (a save, the button or the route) every 60 seconds, to stay inside MaxMind's daily download limit. The route answers 429 `rate_limited` with `retry_after` inside that minute. A save inside the minute waits for the minute to pass and then checks.

A new licence key can take a few minutes to become active at MaxMind. A check made too early fails with "MaxMind rejected the account ID or licence key". Wait a few minutes and press Check now.

### Check the database now

Under Admin, Analytics, the GeoLite2 status shows a Check now button once a licence key is saved. The button runs one check, waits for the result and shows it next to the status. A failed check shows its reason, for example the MaxMind rejection above.

`POST /api/v1/admin/analytics/geo/check` does the same for the root user. It answers when the check ends, after at most 15 minutes, with `status` (`ok`, `failed` or `unconfigured`), `checkedAt`, `updatedAt` and a plain-language `detail`. It returns 409 `geo_not_configured` when no account ID or key is saved, and 404 to every other user. The detail never holds the licence key or the raw reply of MaxMind.

### MaxMind licence and attribution

The GeoLite2 database and the account are MaxMind's, and MaxMind's GeoLite End User License Agreement governs their use. Read it before you switch lookups on. At the time of writing it requires you to attribute your use to MaxMind and gives this example wording: "This product includes GeoLite Data created by MaxMind, available from https://www.maxmind.com." It also requires you to switch promptly to updated databases and to destroy old versions within 30 days of an update. The server's weekly refresh handles the update duty, and it replaces the file rather than keeping old copies in `DATA_DIR/geo`. Check that your backups do not keep old copies of the database longer than the agreement allows, and publish the attribution where the agreement requires it, for example on a privacy page of the sites you host. MaxMind limits GeoLite2 downloads per day.
