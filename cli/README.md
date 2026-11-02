# omnimux CLI

User-level command-line interface for **OmniMux**, a unified AI gateway built on the open-source [new-api](https://github.com/QuantumNous/new-api) project by [QuantumNous](https://github.com/QuantumNous).

OmniMux's consumer product focuses on AI character generation and reference-video recreation. This CLI is an advanced gateway access tool; its command inventory does not promise a complete social-publishing service or require website users to choose model IDs or routing groups. Current product scope: [brand authority](../docs/brand/README.md).

Query models, pricing, groups, and balance, and manage your own API tokens — with an **Agent RPC** JSON envelope (`contract_version: 1` since **0.3.0**) and a hard guarantee that `sk-` keys never touch stdout/stderr.

Breaking wire notes: [CHANGELOG.md](./CHANGELOG.md) · [docs/ERROR_CONTRACT.md](./docs/ERROR_CONTRACT.md).

> **Package name:** published as `@omnimux/cli` on npm.  
> **Binary releases:** https://github.com/laozhong86/omnimux-cli/releases  
> **Source of truth:** this directory in the OmniMux monorepo (`cli/`). The public `omnimux-cli` repo is a release surface only.

## Install

### One-liner (macOS / Linux)

```bash
curl -fsSL https://omnimux.ai/install.sh | bash
omnimux help
```

The installer downloads a prebuilt binary from [laozhong86/omnimux-cli](https://github.com/laozhong86/omnimux-cli/releases) into `/usr/local/bin` or `~/.local/bin`.

> **Agent skills are not bundled with this standalone binary.** To use the optional
> Agent skill packs, install the npm package instead (see below), then run
> `omnimux skill install --user` (or `--project` inside a repository). A source
> checkout can also be used with `OMNIMUX_SKILL_SOURCE=/path/to/cli/skill`.

Version pin:

```bash
curl -fsSL https://omnimux.ai/install.sh | bash -s -- --version 0.5.1
```

### npm / pnpm / bun (Node >= 18 or Bun)

```bash
npm install -g @omnimux/cli    # or: pnpm add -g @omnimux/cli  /  bun add -g @omnimux/cli
omnimux skill install --user   # optional Agent skills -> ~/.agents/skills/<id>
omnimux login                  # browser device-code sign-in
omnimux help
```

### Hackable (inspect published package source)

```bash
npm pack @omnimux/cli
tar -xzf omnimux-cli-*.tgz && cd package
npm link
omnimux help
```

### Windows

```powershell
powershell -c "irm https://omnimux.ai/install.ps1 | iex"
omnimux help
```

Or npm: `npm install -g @omnimux/cli` (needed for the optional Agent skills; the standalone `.exe` does not include them). Or download `omnimux-windows-x64.exe` from GitHub Releases.

## Default instance

The CLI connects to the official instance **`https://omnimux.ai`** out of the box — no configuration needed for public commands like `omnimux pricing`. Self-hosted users (or anyone targeting another instance) override it with `--base-url`, the `OMNIMUX_BASE_URL` env var, or `omnimux config set base_url <url>`.

## Quick start

```bash
# 1. (Optional) Point the CLI at your own instance — it defaults to the
#    official instance https://omnimux.ai
omnimux config set base_url https://your-instance.example.com

# 2. Sign in via browser (device code). Stores system access token + user_id
#    securely (0600 secrets file). Replaces any existing system PAT.
omnimux login

# Manual alternative (if you already have a PAT from the web console):
#   omnimux config set access_token <your-access-token>
#   omnimux config set user_id <your-user-id>

# 3. First commands
omnimux pricing --pretty      # public, no token needed
omnimux models --pretty
omnimux balance --pretty

# 4. (Optional) Agent skills — npm install only
omnimux skill install --user  # ~/.agents/skills/<id>; or --project inside a repo
```

Everything is also overridable per-invocation: `omnimux --base-url https://... models`, or via env vars `OMNIMUX_BASE_URL` / `OMNIMUX_ACCESS_TOKEN` / `OMNIMUX_USER_ID`. Priority: **flag > env > config file > built-in default (`https://omnimux.ai`)**.

## Social migration to 0.4.0

Upgrade with `omnimux update` for binaries or `npm install -g @omnimux/cli@0.4.0` for npm. Update scripts as well: `social accounts`, `connect`, `disconnect`, `post`, and `post-status` all require `--provider zernio|tiktok_direct`. No source is inferred. For example:

```bash
omnimux social accounts --provider zernio
omnimux social accounts --provider tiktok_direct
omnimux social post-status <post-id> --provider zernio --no-refresh
```

Use the provider that owns the account/post. Missing or unknown CLI providers fail before HTTP; the gateway rejects invalid providers with 400 and cross-source resources with 409. The Agent RPC envelope remains version 1. See [CHANGELOG.md](./CHANGELOG.md) for all changes since 0.3.0, including credential storage and hosted commands.

## Commands

| Command | Description |
|---------|-------------|
| `omnimux models` | Models available to you |
| `omnimux pricing` | Public model price list (no token needed) |
| `omnimux groups` | Your groups and ratios |
| `omnimux balance` | Balance and usage (adds `derived.*_usd` fields) |
| `omnimux tokens` | List API tokens (keys always masked) |
| `omnimux tokens create <name> [--group=g] [--quota=usd]` | Create a token; the key is never displayed. Output includes `token_id` — from the server response when available, otherwise resolved via a follow-up masked list call |
| `omnimux tokens switch-group <id> <group>` | Move a token to another group |
| `omnimux tokens copy <id>` | Copy the real key straight to the clipboard |
| `omnimux tokens apply <id> <file>` | Replace `__OMNIMUX_TOKEN_<id>__` in a file with the real key, atomically |
| `omnimux tokens exec <id> [--timeout=sec] -- <cmd...>` | Run a command with `__OMNIMUX_TOKEN_<id>__` substituted; output sanitized. Child timeout: 300s default, override with `--timeout=<sec>` |
| `omnimux login [--no-open]` | Browser device-code login; stores system access token |
| `omnimux logout` | Clear stored access token and user_id |
| `omnimux scan-config <file>` | Show a config file with secrets redacted (best-effort) |
| `omnimux config [get\|set\|unset]` | View/set `base_url`, `user_id`, `access_token`, `auto_update` |
| `omnimux update [--check] [--force]` | Self-update (npm global or release binary). Also migrates legacy `geminix.cc` `base_url` → official default |
| `omnimux upgrade` | Alias for `update` |
| `omnimux help` | Usage overview |

Global flags: `--base-url <url>`, `--pretty`, `--version`, `--check` / `--force` (for `update`), `--no-auto-update`.

### Auto-update (default on)

On normal command runs the CLI may self-update in the background (at most once per 24h):

- **Default:** enabled
- **Disable permanently:** `omnimux config set auto_update false`
- **Disable via env:** `export OMNIMUX_AUTO_UPDATE=0`
- **Disable once:** `omnimux --no-auto-update <command>`
- **Re-enable:** `omnimux config set auto_update true` or `omnimux config unset auto_update`
- Notices go to **stderr** only (stdout stays JSON). The current process finishes on the old version; the next run uses the new one.
- Skipped for `update`/`upgrade`, source checkouts, and when offline/throttled.

## Security

- **Keys never print.** Token list/show responses mask `key` fields; the real key flows only from the API into the clipboard (child-process pipe), a placeholder in your config file (atomic write), or a child command (output re-sanitized). No command can be made to echo a full `sk-` key.
- **Secrets are stored properly.** `access_token` goes to `~/.config/omnimux/secrets.json` (mode `0600`) on every platform; never to `config.json`.
- **Output contract (Agent RPC).** Success: stdout `{ ok, data, meta }`; errors: stderr `{ ok:false, error, meta }` (typed `type`/`subtype`/`recovery`). Exit: `0` ok, `1` api, `2` config/auth, `3` usage, `4` network, `5` internal, `10` confirmation. See `docs/ERROR_CONTRACT.md`. Discover commands with `omnimux schema`.
- **User-level only.** The CLI ships no admin endpoints; `test/no-admin.test.js` scans the source and fails the build if any admin route prefix appears. For instance operations/administration, use the separate `newapi` agent skill.

## AI editor skill

Package ships a **flat** skill tree under `skill/` (one directory per Agent skill). The tree ships in the npm package and source checkout; the standalone installer binary does not embed it. Install:

```bash
omnimux skill install --user       # ~/.agents/skills/omnimux + recreate-viral-video + …
omnimux skill install --project    # <repo>/.agents/skills/<id>
omnimux skill status
omnimux skill update --project     # also migrates legacy whole-tree installs from 0.2.6
```

| Mode | Behavior |
| --- | --- |
| **link** (default) | Each `.agents/skills/<id>` → package `skill/<id>` |
| **copy** | Snapshot + `.omnimux-skill-meta.json`; run `skill update` after CLI upgrades |

| skill id | Role |
| --- | --- |
| `omnimux` | Shared: CLI, auth, tokens, secrets |
| `recreate-viral-video` | Business: viral/reference video safe adaptation |

There is **no** nested `packs/` layer — Agents discover skills as siblings.

If you installed the standalone binary, switch to the npm package (or set `OMNIMUX_SKILL_SOURCE` to a source checkout's `cli/skill`) before running `skill install`. Restart your AI agent afterwards so it rescans skills.

## Development

```bash
cd cli
bun test                    # unit + guard tests (contract tests skip without a live instance)
bun src/bin.js help         # run from source
bun run build:binaries      # dist/ single-file binaries for 4 platforms (requires bun)
```

### Releasing (maintainers / Agents)

See `cli/AGENTS.md` and `.agents/skills/cli-release/SKILL.md`.

- Bump `package.json` version → tag **`cli-vX.Y.Z`** on monorepo → CI builds binaries and publishes to `laozhong86/omnimux-cli` (+ optional npm).
- Do not tag CLI releases as bare `vX.Y.Z` (gateway workflows).

Live-instance contract tests:

```bash
OMNIMUX_TEST_BASE_URL=http://localhost:7001 \
OMNIMUX_ACCESS_TOKEN=... OMNIMUX_USER_ID=... \
bun test test/contract.test.js
```

## License & attribution

This project is a fork of [new-api](https://github.com/QuantumNous/new-api), Copyright © QuantumNous and contributors, licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See `LICENSE` and `NOTICE` at the repository root. All upstream attribution is preserved as required.

## Character and reference-video creation

The account-level `influencer` commands share characters and generation history with the web workbench. Read `omnimux influencer options` first: the configured service must support and enable creation. Old credentials must explicitly authorize the new creation permissions through login.

```bash
omnimux influencer character create --brief 'adult character in a blue coat' --request-id character-001 --dry-run
omnimux influencer character create --brief 'adult character in a blue coat' --request-id character-001 --yes
omnimux influencer character list
omnimux influencer character multiview <character_task_id> --request-id views-001 --yes
omnimux influencer video create --character <character_task_id> --video reference.mp4 --request-id video-001 --yes
omnimux influencer task wait <task_id> --timeout 600
omnimux influencer task download <task_id> --output ./result.mp4
omnimux influencer request get video-001
```

Creation consumes account credits/subscription quota; use `--yes` only with matching authorization. It returns acceptance immediately, not completed media. `task get` reads any owned status; `task wait` succeeds only for generation success with an artifact. Timeout/interruption leaves the cloud task running.

Types: `normal` (default), `freak`, `total`, `cats`, `dogs`. `--selection` is a JSON category-to-option-ID-array object obtained from options, and `--reference-image` adds an optional style reference. Video uses `--character` and/or ordered repeated `--image` references (1–30 total), with `--video` or `--video-task`. Defaults are `motion` and `480p`; 480p supports `16:9`, `9:16`, `3:4`, and 720p supports `16:9`, `9:16`. Replace mode preserves the reference aspect. Local images are capped at 5 MiB each; reference video at 100 MiB. The server inspects real duration (4–30 seconds), without a client media-tool dependency. `--input` accepts product JSON fields without overlapping explicit inputs. No consumer model/group selection or duration override is supported.

A private 0600 recovery record is atomically stored under the configuration directory, partitioned by server/account, before uploads. Retain the client `request_id`; after uncertain submission, lookup precedes any retry, and a confirmed missing request can only reuse the exact persisted body. Changed intent/files with the same ID fail. Never switch to a new request ID to recover an unknown outcome. `--dry-run` stays offline and performs only local validation, with server media inspection marked pending.

Downloads require `--output`, refuse overwrite unless `--overwrite` is explicit, stream through a temporary sibling, and verify media/length before completion. Signed media URLs are not printed or journaled and account credentials are not sent to storage or forwarded through redirects. See `omnimux schema`, the [creation guide](skill/omnimux/docs/actions-influencer.md), and the [error contract](docs/ERROR_CONTRACT.md).
