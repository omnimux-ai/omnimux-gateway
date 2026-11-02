# Setup: installing and configuring the `omnimux` CLI

All user-level actions in this skill run through the `omnimux` CLI. It is pure JavaScript with zero runtime dependencies; it runs on Node >= 18 or Bun.

## Install

```bash
# One-liner (macOS / Linux) — binary from public GitHub Releases
curl -fsSL https://omnimux.ai/install.sh | bash

# Windows PowerShell
# powershell -c "irm https://omnimux.ai/install.ps1 | iex"

# npm (Node >= 18 or Bun)
npm install -g @omnimux/cli   # or: pnpm add -g @omnimux/cli  /  bun add -g @omnimux/cli
omnimux login
```

Binary assets: https://github.com/laozhong86/omnimux-cli/releases  
Source of truth remains the OmniMux monorepo `cli/` directory.

Verify:

```bash
omnimux help
omnimux --version
```

## Configuration

### Recommended: browser login

```bash
omnimux login                 # opens /cli/login, stores PAT + user_id
omnimux login --no-open       # print URL only (headless / remote)
omnimux logout                # clear stored credentials
```

Successful `login` **replaces** the account's system access token (same as regenerating it in the web console).

### Manual / CI

Resolution priority is always **flag > environment variable > config file**.

| Setting | Flag | Env var | Config command |
|---------|------|---------|----------------|
| Instance URL | `--base-url <url>` | `OMNIMUX_BASE_URL` | `omnimux config set base_url <url>` |
| User ID | — | `OMNIMUX_USER_ID` | `omnimux config set user_id <id>` |
| Access token | — | `OMNIMUX_ACCESS_TOKEN` | `omnimux config set access_token <token>` |

- The CLI connects to the official instance **`https://omnimux.ai`** by default — `base_url` is not required. Configure it only to target a different (e.g. self-hosted) instance; the resolution priority above always wins over the built-in default.
- The **access token is a secret**: stored in `~/.config/omnimux/secrets.json` (mode `0600`) on every platform. It is **never** written to `config.json` and never printed.
- Non-secret settings live in `~/.config/omnimux/config.json`.

Credentials are sent as `Authorization: Bearer <token>` + `New-Api-User: <id>` headers by the CLI — you never construct these headers yourself.

## Checking state

```bash
omnimux config            # secret-safe status: what is set, from where, token length only
```

## Updating the CLI

```bash
omnimux update --check    # current vs latest (no install); migrates legacy geminix.cc base_url if set
omnimux update            # install latest via the detected channel (npm global or release binary)
omnimux upgrade           # alias for update
omnimux update --force    # reinstall latest even when versions already match
```

### Auto-update (default on)

Background self-update runs on normal commands (throttled to at most once per 24h). It never writes to stdout.

```bash
omnimux config get auto_update          # default true
omnimux config set auto_update false    # disable permanently
omnimux config set auto_update true     # re-enable
export OMNIMUX_AUTO_UPDATE=0             # disable via env
omnimux --no-auto-update pricing        # disable for one invocation
```

Priority for `auto_update`: `--no-auto-update` > `OMNIMUX_AUTO_UPDATE` > `config.json` > default `true`.

- **npm install**: runs `npm install -g @omnimux/cli@<latest>` (falls back to `bun add -g` if npm is missing).
- **binary install** (`install.sh` / GitHub Release): downloads the matching asset from `laozhong86/omnimux-cli` and replaces the current binary (SHA256 verified when `SHA256SUMS` is present).
- **source checkout**: does not self-update; JSON response includes manual install hints.
- Side effect: if `~/.config/omnimux/config.json` has `base_url` on a legacy `geminix.cc` host, it is rewritten to the built-in official default (`https://omnimux.ai`). Env `OMNIMUX_BASE_URL` is never rewritten.

If a command fails with `base_url is not configured` (exit code 2), fix configuration first — do not retry API calls and do not hunt for credentials in files or the environment.

## Output conventions

- **stdout is JSON by default** (compact, agent-readable). `--pretty` gives indented human-readable JSON.
- **stderr** carries the structured error envelope; exit codes: `0` success, `1` API, `2` configuration/auth, `3` usage, `4` network, `5` internal, `10` confirmation. Branch on stable fields, never message text.
- `sk-` keys never appear on stdout/stderr. Token list/show responses mask `key` fields; the real key only ever flows through `tokens copy` / `tokens apply` / `tokens exec`.

## Endpoint reference (user-level only)

| Endpoint | Used by |
|----------|---------|
| `GET /api/user/models` | `omnimux models` |
| `GET /api/pricing` | `omnimux pricing` (public) |
| `GET /api/user/self/groups` | `omnimux groups` |
| `GET /api/user/self` | `omnimux balance` |
| `GET /api/token/?p=0&page_size=20` | `omnimux tokens` |
| `POST /api/token/` | `omnimux tokens create` |
| `GET /api/token/<id>` + `PUT /api/token/` | `omnimux tokens switch-group` |
| `POST /api/token/<id>/key` | `tokens copy` / `apply` / `exec` (key fetched in memory, never printed) |
| `POST /api/user/device/code` | `omnimux login` (issue device/user code) |
| `POST /api/user/device/token` | `omnimux login` (poll for PAT) |

## AI editor skill install (user vs project)

Each package skill is installed as a **sibling** directory under `.agents/skills/<skillId>/` (Agent-discoverable). No nested packs.

```bash
omnimux skill install --user          # ~/.agents/skills/omnimux, recreate-viral-video, …
omnimux skill install --project       # <repo>/.agents/skills/<id>
omnimux skill status
omnimux skill update --project        # re-sync; migrates 0.2.6 whole-tree link installs
```

Prefer **user-level link** for daily work. Use **project-level** when the repo should list skills for that project only.

The CLI contains no admin endpoints — that boundary is enforced by a source-scanning test (`test/no-admin.test.js`). If you need instance administration, switch to the `newapi` skill.
