# OpenHere agent skill

`openhere/` is an [Agent Skills](https://agentskills.io/specification) package that teaches an AI coding agent to publish built static sites to an OpenHere instance. It holds a `SKILL.md`, three scripts (`publish.sh`, `auth.sh`, `status.sh`) and two reference files.

## Install

With the [`skills` CLI](https://github.com/vercel-labs/skills), which installs for Claude Code, Codex, Cursor and other agents:

```sh
npx skills add zachlagden/openhere --skill openhere -g
```

`-g` installs for your user (for example `~/.claude/skills/openhere`). Leave it out to install into the current project. The CLI finds the skill because it lives at `skills/openhere/SKILL.md`. It clones the repository with the Git or GitHub CLI credentials you already have, so a private repository works for a signed-in user. Anyone else needs the repository to be public.

To install by hand, copy the folder to where your agent reads skills:

```sh
cp -r skills/openhere ~/.claude/skills/openhere
```

Other agents use their own folder, such as `~/.agents/skills/openhere` for Codex. Keep the folder name `openhere`, and keep `scripts/` executable.

## What it needs

- `curl` and `jq`, plus `sha256sum` or `shasum`. `file` is used when present to detect unusual content types.
- An OpenHere instance and an account on it. The agent signs in with an emailed code (the instance needs email configured) or with an API key you create in the dashboard under Settings, API keys. It saves the key to `~/.openhere/credentials` with mode 0600. `OPENHERE_URL` and `OPENHERE_API_KEY` override the file.

The dashboard's "Copy agent setup prompt" button gives your agent these steps with your instance URL filled in.

## Test it

Run the scripts against a throwaway instance on your machine:

```sh
PUBLIC_URL=http://localhost:8080 DATA_DIR=/tmp/openhere-data go run ./cmd/openhere
```

Complete setup in the dashboard with the logged token, create an API key, then:

```sh
printf '%s' "$KEY" | bash skills/openhere/scripts/auth.sh save-key --instance http://localhost:8080
bash skills/openhere/scripts/publish.sh path/to/built/site --dry-run
bash skills/openhere/scripts/publish.sh path/to/built/site
bash skills/openhere/scripts/status.sh
```

Lint the scripts with `shellcheck -x skills/openhere/scripts/*.sh`. Validate the skill with the `skills-ref` library (`skills-ref validate skills/openhere`).

Plain `http://` instance addresses are refused unless the host is `localhost`, so a real instance needs HTTPS.
