# Email, invites and password resets

## Email

Email is optional. Without it, the server still works, and each feature that needs email says so: the invite response sets `emailAvailable` to false and still returns the invite link, password reset requests send nothing, the viewer email code stays off and agent email sign-in returns `email_not_configured`.

The root user sets it up with `PATCH /api/v1/admin/settings`.

For SMTP:

```sh
curl -X PATCH http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"email.provider":"smtp","email.fromAddress":"noreply@example.com","email.fromName":"Acme Ltd",
       "email.smtp.host":"smtp.example.com","email.smtp.port":587,"email.smtp.security":"starttls",
       "email.smtp.username":"alex","email.smtp.password":"change-me"}}'
```

For Resend:

```sh
curl -X PATCH http://localhost:8080/api/v1/admin/settings \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"settings":{"email.provider":"resend","email.fromAddress":"noreply@example.com","email.resend.apiKey":"re_example"}}'
```

Email counts as configured only when the provider is set and its required fields are present: SMTP needs a host, a valid port, a security mode and the from address (and a password when it has a user name), Resend needs an API key and the from address. `POST /api/v1/admin/email/test` (root only, at most 5 per minute) sends a test message to the root user's address and returns either `{"sent":true}` or a short reason such as "the server rejected the username or password". The reason never contains a secret or the server's raw reply.

Messages are plain text. Links in them start with `PUBLIC_URL`.

The provider secrets are write-only and stored encrypted. See [Security](security.md#secrets-at-rest).

## Invites

An invite creates an instance account. Nobody can sign up on their own. By default only the root user can invite. Set `invites.whoCanInvite` to `workspaceAdmins` to let a workspace admin invite into a workspace they administer. Creating shared workspaces has its own setting, `workspaces.whoCanCreate`.

`POST /api/v1/invites` takes an email, an optional `workspace` (slug or id of a shared workspace) with a workspace-wide `role` (`admin`, `write` or `read`, default `read`), and `sendEmail`. The response always carries the link `PUBLIC_URL/invite/<token>`. The server stores only a hash of the token, so this response is the one time the link is shown. If email is configured and `sendEmail` is true, the server also emails the link. `emailAvailable` and `emailSent` tell the caller what happened, so a client can hide its email option when `emailAvailable` is false. An email address that already has an account returns 409, and the message suggests adding the user to the workspace instead.

An invite expires after 7 days and works once. `GET /api/v1/invites` lists invites (all of them for the root user, the ones you sent for anyone else). `DELETE /api/v1/invites/{id}` revokes one. `POST /api/v1/invites/{id}/resend` revokes the old link and issues a new one.

The invitee opens the link. `GET /api/v1/invites/{token}` returns only the invited email and the workspace name, and `POST /api/v1/invites/{token}/accept` with a password and an optional display name creates the user, their personal workspace and the membership in one step, then signs them in. Unknown, expired, revoked and used tokens all return the same 404. These two routes need no sign-in and are rate limited. The page at `/invite/<token>` is part of the dashboard.

## Users and password resets

The root user lists accounts with `GET /api/v1/admin/users` and disables or enables one with `PATCH /api/v1/admin/users/{id}` and `{"disabled":true}`. A disabled user cannot sign in, and their sessions and API keys stop working at once. The root user cannot disable themselves, and the last root user cannot be disabled. There is no way to delete a user.

Anyone can ask for a reset link with `POST /api/v1/auth/password-reset/request`. The response is always the same, whether or not the account exists. The server sends the link only when email is configured and the account exists and is enabled. `POST /api/v1/auth/password-reset/confirm` with the token and a new password sets the password. A link works once and expires after an hour. A reset ends all of the user's sessions and keeps their API keys. Both routes are rate limited per IP address, and the request route per email address too.

Without email, the root user generates a link with `POST /api/v1/admin/users/{id}/password-reset`. The response shows the link once, and the root user passes it on.
