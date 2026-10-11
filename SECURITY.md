# Security policy

## Reporting a vulnerability

Report security problems privately. Do not open a public issue or pull request for one.

Send the report to zach@zachlagden.uk.

Include what you found, the version or commit, the steps to reproduce it and the effect you expect it to have. If you found it on a running instance, say which settings were involved (hosting mode, access mode, proxy setup) and leave out real credentials and personal data.

## Supported versions

Only the latest release and the `main` branch receive fixes.

## Scope

In scope: the server, the dashboard, the Dockerfile and the release workflow in this repository. The security model, including what is deliberately public and the known limits, is in [docs/security.md](docs/security.md). Read it before you report, because some behaviour it lists is by design, such as a new site being public by default and a gate page revealing that a slug exists.

Out of scope: a weakness in a third-party service you run alongside the server (a reverse proxy, a database, a DNS provider), a malicious account holder using the access you gave them, and denial of service through unlimited authenticated publishing.
