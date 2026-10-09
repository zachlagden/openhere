package hosting

const gatePageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{.Title}}</title>
<style>
:root{color-scheme:light dark;--bg:#f5f5f2;--panel:#fff;--ink:#1d1d1b;--muted:#5f5f59;--line:#d8d8d2;--accent:#1f4fd8;--accent-ink:#fff;--alert:#fdecea;--alert-ink:#7a1b12;--note:#e8eefc;--note-ink:#15306e}
@media (prefers-color-scheme:dark){:root{--bg:#161614;--panel:#20201d;--ink:#ecece6;--muted:#a3a39b;--line:#383833;--accent:#7da0ff;--accent-ink:#10131c;--alert:#3b1c18;--alert-ink:#ffc9c2;--note:#1b2542;--note-ink:#c7d6ff}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:1rem;background:var(--bg);color:var(--ink);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
main{width:100%;max-width:26rem;padding:1.75rem;background:var(--panel);border:1px solid var(--line);border-radius:.5rem}
h1{margin:0 0 1rem;font-size:1.25rem;line-height:1.3}
p{margin:0 0 1rem}
.muted{color:var(--muted);font-size:.9rem}
.notice,.info{padding:.6rem .8rem;border-radius:.375rem}
.notice{background:var(--alert);color:var(--alert-ink)}
.info{background:var(--note);color:var(--note-ink)}
form{margin:0 0 1rem}
label{display:block;margin-bottom:.25rem;font-weight:600}
input[type=password],input[type=email],input[type=text]{display:block;width:100%;margin-bottom:.75rem;padding:.55rem .65rem;font:inherit;color:inherit;background:transparent;border:1px solid var(--line);border-radius:.375rem}
button,.button{display:inline-block;padding:.55rem 1rem;font:inherit;font-weight:600;color:var(--accent-ink);background:var(--accent);border:0;border-radius:.375rem;text-decoration:none;cursor:pointer}
button:focus-visible,.button:focus-visible,input:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
hr{border:0;border-top:1px solid var(--line);margin:1.25rem 0}
</style>
</head>
<body>
<main>
<h1>{{.Heading}}</h1>
{{- if .Intro}}
<p>{{.Intro}}</p>
{{- end}}
{{- if .Message}}
<p class="{{if .Info}}info{{else}}notice{{end}}" role="{{if .Info}}status{{else}}alert{{end}}">{{.Message}}</p>
{{- end}}
{{- if .Password}}
<form method="post" action="{{.UnlockURL}}">
<input type="hidden" name="token" value="{{.Token}}">
<input type="hidden" name="return" value="{{.Return}}">
<label for="password">Password</label>
<input id="password" name="password" type="password" autocomplete="current-password" required autofocus>
<button type="submit">Continue</button>
</form>
{{- end}}
{{- if .SignIn}}
<p>If you have an account on this server, sign in with it.</p>
<p><a class="button" href="{{.HandoffURL}}">Sign in with your account</a></p>
<hr>
{{- end}}
{{- if .EmailForm}}
<form method="post" action="{{.CodeURL}}">
<input type="hidden" name="token" value="{{.Token}}">
<input type="hidden" name="return" value="{{.Return}}">
<label for="email">Email address</label>
<input id="email" name="email" type="email" autocomplete="email" required>
<button type="submit">Email me a code</button>
</form>
<p class="muted">If your address has been added to this site, we send an 8-digit code that works for 10 minutes.</p>
{{- end}}
{{- if .NoEmail}}
<p class="muted">Email sign-in is not available on this instance.</p>
{{- end}}
{{- if .CodeEntry}}
<form method="post" action="{{.VerifyURL}}">
<input type="hidden" name="token" value="{{.Token}}">
<input type="hidden" name="return" value="{{.Return}}">
<input type="hidden" name="email" value="{{.Email}}">
<label for="code">8-digit code</label>
<input id="code" name="code" type="text" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{8}" maxlength="8" required autofocus>
<button type="submit">Continue</button>
</form>
{{- end}}
</main>
</body>
</html>
`
