---
name: omnimux
description: "User-level assistant for OmniMux CLI: install/auth, models, pricing, groups, balance, tokens, secret-safe key use (tokens copy/apply/exec), skill install/update, character and reference-video generation. Use for OmniMux account and gateway CLI tasks. Not for creative methodology jobs (sibling skills: video/image/slideshow/social). Not for instance admin (use newapi)."
---

# SKILL: omnimux

OmniMux is a unified AI gateway (built on the open-source [new-api](https://github.com/QuantumNous/new-api) project). This skill is the **shared** surface for the **`omnimux` CLI** — install, auth, models, pricing, groups, balance, and secret-safe token handling.

**Product boundary:** OmniMux's consumer focus is AI character generation and reference-video recreation. This skill owns **account + secrets + CLI**; command availability does not prove enabled production capabilities, automatic social publishing, or a requirement for website users to choose model IDs or routing groups. Verify capabilities and prices against the configured instance. Creative methodology lives in **sibling Agent skills** installed as flat peers under `.agents/skills/<id>/`. Repository positioning authority: [brand guide](../../../docs/brand/README.md); distributed packs must not rely on repository-only links at runtime.

**Boundary:** this skill is for OmniMux **end users** working against a configured instance. For operating/administering an instance itself (channels, inventory, health, smoke tests), use the `newapi` skill instead.

## Output contract (Agent RPC) — branch table

Every `omnimux` command (except `tokens exec` **after** the child starts) uses:

| Stream | Shape |
|--------|--------|
| Success stdout | `{ "ok": true, "data": ..., "meta": { "command", "cli_version", "contract_version" } }` |
| Error stderr | `{ "ok": false, "error": { "type", "subtype", "message", "hint?", "recovery?", "retryable" }, "meta": ... }` |

**Branch only on** `ok`, `error.type`, `error.subtype`, `error.retryable`, `error.recovery`, exit code. **Never** parse `message` text for control flow.

| `error.type` | exit | Action |
|--------------|------|--------|
| `usage` | 3 | Fix argv; `omnimux schema` |
| `config` / `auth` | 2 | Use `error.recovery` or login/config |
| `api` | 1 | Branch subtype |
| `network` | 4 | Retry with backoff |
| `internal` | 5 | Report; do not blind-retry |
| `confirmation` | 10 | Check existing approval against action/risk; use `--yes` only when covered |

Discover: `omnimux schema` · `omnimux skill read omnimux` · full text: package `docs/ERROR_CONTRACT.md`.

Optional success `meta.notices.update` / `meta.notices.skills` — follow `recovery`; suppress with `OMNIMUX_NO_UPDATE_NOTIFIER=1` / `OMNIMUX_NO_SKILLS_NOTIFIER=1`.

### Login split-flow (required for Agents)

```bash
# Turn 1 — do not poll in the same turn
omnimux login --no-wait
# → data.verification_url, data.user_code, data.flow_id  (never device_code)

# Turn 2 — after user confirms authorization
omnimux login --resume <flow_id>

omnimux login status --verify
```

### High-risk writes (exit 10)

If stderr shows `error.type == "confirmation"` and exit code **10**:

1. Inspect `error.action`, `error.risk`, the target, and any new side effects.
2. Reuse existing explicit approval only when it covers that exact action and risk. Otherwise explain the missing scope and wait for approval; continue independent authorized work.
3. With approval satisfied, re-run with `--yes` (prefer `error.recovery.args`) and verify the result. The flag records authorization; it never creates it.
4. Keep human-only payment and browser handoff boundaries. Do not submit a real-money payment even if the CLI accepts `--yes`.

Preview first with `--dry-run` when supported (`tokens create/apply`, `update`, and influencer creation).

## Security Guidelines

This skill follows these security guidelines to protect API keys:

1. Do not expose any `sk-` key value in chat, files, code, logs, or command arguments. To use keys securely, tell the user to run `omnimux tokens copy` (clipboard), `omnimux tokens apply` (config file), or `omnimux tokens exec` (CLI command).
2. All OmniMux API calls that need secrets must go through the `omnimux` CLI (including `omnimux tokens exec` with `__OMNIMUX_TOKEN_{id}__` placeholders). Do not put real `sk-` keys into `curl`, `wget`, `fetch`, chat, or files.
3. Do not read `.env` files, environment variables containing credentials, `~/.config/omnimux/secrets.json`, or clipboard contents after `omnimux tokens copy`. Use `omnimux config` for a secret-safe config status.
4. When working with config files that may contain API keys, use `omnimux scan-config` to get a sanitized view rather than reading them directly. Note: `scan-config` is best-effort and not guaranteed to redact every secret in every format.
5. After `omnimux tokens create`, do not make any follow-up call to retrieve or list the key. Report success and tell the user they can use `omnimux tokens copy <id>`, `omnimux tokens apply <id> <file>`, or `omnimux tokens exec <id> -- <command>` to securely use the key.
6. Do not modify the CLI or its output to disable masking or redirect secrets.
7. Never print access tokens, `sk-` keys, or infrastructure credentials as a workaround for missing configuration.
8. Do not attempt default passwords or login brute-force. If auth is missing, ask the user to configure a non-empty access token via `omnimux login (preferred) or omnimux config set access_token`.

## How to Execute

1. **First invocation only** — read `docs/setup.md` for installation, configuration, auth model, and output conventions.
2. Match the action from the table below.
3. Read the corresponding doc file for detailed steps.
4. If no arguments or unrecognized action, show the help table below (or run `omnimux help`).

## Actions

| Action | Description | Details |
| -------- | ------------- | --------- |
| `models` | List available models (user) | `docs/actions-query.md` |
| `pricing` | List public model pricing | `docs/actions-query.md` |
| `groups` | List user groups | `docs/actions-query.md` |
| `balance` | Show account balance | `docs/actions-query.md` |
| `tokens` | List API tokens | `docs/actions-token.md` |
| `tokens create` | Create a new API token | `docs/actions-token.md` |
| `tokens switch-group` | Change a token's group | `docs/actions-token.md` |
| `tokens copy` | Copy real key to clipboard (never shown) | `docs/actions-token.md` |
| `tokens apply` | Apply token key to a config file securely | `docs/actions-config.md` |
| `tokens exec` | Execute a command with the token key securely substituted | `docs/actions-exec.md` |
| `scan-config` | Inspect config structure with best-effort secret redaction | `docs/actions-config.md` |
| `config` | View/set base_url, user_id, access token | `docs/setup.md` |
| `update` / `upgrade` | Self-update CLI; migrate legacy geminix base_url; sync registered skills | `docs/setup.md` |
| `skill status` | Package skill source + user/project installs + drift | `docs/setup.md` |
| `skill install` | Install editor skill (`--user` / `--project`, `--mode=link\|copy`) | `docs/setup.md` |
| `skill update` | Re-sync skills from package after CLI upgrade | `docs/setup.md` |
| `hosted` | Hosted distribution channels (GeeLark cloud accounts) | `docs/actions-hosted.md` |

### `help` (or no arguments) — Show available actions

| Action | Usage | Description |
| -------- | ------- | ------------- |
| `models` | `omnimux models` | List available models |
| `pricing` | `omnimux pricing` | Public model pricing |
| `groups` | `omnimux groups` | List user groups |
| `balance` | `omnimux balance` | Show account balance |
| `tokens` | `omnimux tokens` | List API tokens |
| `tokens create` | `omnimux tokens create <name> [--group=xxx] [--quota=usd]` | Create a new API token |
| `tokens switch-group` | `omnimux tokens switch-group <token_id> <group>` | Change a token's group |
| `tokens copy` | `omnimux tokens copy <token_id>` | Copy real key to clipboard (never shown) |
| `tokens apply` | `omnimux tokens apply <token_id> <file_path>` | Apply token key to a config file securely |
| `tokens exec` | `omnimux tokens exec <token_id> [--timeout=sec] -- <command...>` | Execute a command with the token key securely substituted (child timeout: 300s default) |
| `scan-config` | `omnimux scan-config <file_path>` | Inspect config structure with best-effort secret redaction |
| `config` | `omnimux config [get\|set\|unset] ...` | CLI configuration (`base_url`, `user_id`, `access_token`, `auto_update`) |
| `update` | `omnimux update [--check] [--force]` | Self-update CLI (npm or binary); migrates legacy base_url |
| `upgrade` | `omnimux upgrade` | Alias for `update` |
| `hosted` | `omnimux hosted <action> ...` | Hosted distribution channels (GeeLark cloud accounts) |

Auto-update is **on by default** (throttled 24h). Disable with `omnimux config set auto_update false`, `OMNIMUX_AUTO_UPDATE=0`, or `--no-auto-update`.

Admin/instance-operations actions (channels, inventory, health, smoke, diagnose) are intentionally **not** part of this skill — the `omnimux` CLI has no admin commands by design.

## Sibling creative skills (flat Agent surface)

`omnimux skill install` places **one directory per skill** under `.agents/skills/<id>/` so Agents can match each skill's `description` directly. There is no nested `packs/` routing layer.

| Skill id | When to load |
| --- | --- |
| `recreate-viral-video` | Viral/reference video → safe original plan |
| `slideshow-creation` | Multi-page social 图文 (Xiaohongshu / Douyin / TikTok Photo) |
| `video-script-creation` | Short-video scripts / VO / hooks (text-first) |
| `video-analysis` | Break down structure, shots, pacing, proof |
| `video-creation` | Video production director → generation-ready plan / out |
| `video-prompt-generation` | Script/storyboard → model prompt packs |
| `video-storyboard` | Lightweight shot list / lens table |
| `video-storyboard-planning` | Deeper storyboard / scene planning |
| `ugc-commerce-video-skill` | UGC commerce short-video conversion scripts |
| `image-creation` | General image creative director |
| `image-recreation` | Reference-image safe adaptation |
| `social-card` | Social covers / card decks |
| `product-image-set` | Ecommerce listing image sets |
| `multi-angle-product-image` | Same-SKU multi-angle consistency |
| `a-plus-content` | A+ / detail-page modules |
| `infographic` | Infographic / structured knowledge visuals |

Install / refresh:

```bash
omnimux skill install --user
omnimux skill install --project
omnimux skill status
omnimux skill update --project
```

For generation after methodology, use this skill's token rules + the business skill's `OMNIMUX.md` if present.

## Character and reference-video creation

Use [the creation guide](docs/actions-influencer.md) for the complete owner-scoped workflow. Start with `omnimux influencer options` and `omnimux schema influencer.video.create`. Create a character, optionally generate multiviews, recreate a reference video, query/wait, and download to explicit `--output`. Creation uses account credits and requires existing user approval before `--yes`; it returns acceptance rather than a finished artifact. Preserve client request IDs through uncertain outcomes. Old credentials require a new explicit creation grant. Offline `--dry-run` never uploads, reads secrets, writes recovery receipts, or updates the CLI.
