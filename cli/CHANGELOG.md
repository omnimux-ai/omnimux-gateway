# Changelog — `@omnimux/cli`

## 0.5.1

### Fixed

- Restored complete `SKILL.md` bodies for the `image-creation` and `social-card` skills. The 0.5.0 npm package shipped frontmatter-only files without a closing delimiter, so Agents could not load either skill.
- Rewrote the remaining host-tool references in those two packs (`render-html-to-png`, `workflow-slideshow-persist`, `generate-image`, …) as visible-capability contracts bound to `OMNIMUX.md`.
- Removed the vendored product-skill copies from the monorepo's `.agents/skills/`; `cli/skill/` is the single source again, which prevents the drift that made `.agents/skills/omnimux` lag behind the package.

## 0.5.0

### Added

- Character/video creation commands for querying product options, generating and reusing character sheets, optional multiview sheets, creating reference-driven videos, and reading account-owned task history and artifacts.
- Durable local request receipts, server-backed duplicate-submission protection, bounded resumable waits and verified file downloads without default overwriting.
- Explicit creator authorization through the existing device login. Existing credentials require a newly approved grant; upgrading alone does not add permission.
- Signed artifact addresses remain private, and downloads reject partial responses and incomplete files.
- Includes the inspiration sharing/category commands from the previously unpublished 0.4.1–0.4.4 source updates, plus voice discovery.

### Availability

- Requires a gateway exposing `/api/influencer/v1` and enabling new CLI creations. Read `influencer options` before creating; command availability does not prove that generation is currently enabled.
- Reference videos must be 4–30 seconds. Creation acceptance is not delivery; query or wait for the original task and download its completed artifact.

## 0.4.4

### Added

- `omnimux inspiration categories [--group <key>]` command to list curated inspiration community categories with explicit required and optional field metadata.
- Updated `omnimux inspiration publish` and `share` help text with explicit `(必填)` and `(可选)` indicators on every option.

## 0.4.3

### Added

- `omnimux inspiration publish` automatically uploads local files when `--media <path>` or `--cover <path>` points to a local file. Local media files are streamed directly to Cloudflare R2 (`omnimux-files`) via the gateway file service before publishing.
- Automatic media type detection from local file extensions (e.g. `.png`, `.jpg`, `.webp` default to `image`, else `video`).
- `uploadGatewayFile` helper in `src/lib/upload.js` for multipart streaming file uploads to the gateway.

## 0.4.2

### Added

- `omnimux inspiration share <inspiration_id>` — turn a row that already lives in the cloud inspiration library into a direct link. Cloud rows are permanent, so the link does not expire.
- `--expire 3days|forever` on `publish` and `share` to override the default window.
- `--id` / `--inspiration-id` accepted as the id spelling on `share`.

### Changed

- `publish` and `share` now call the single share entry (`POST /api/inspiration/v1/share`) with an explicit `source`; the previous `/publish` path remains available on the gateway as a compatibility alias.

## 0.4.1

### Added

- `omnimux inspiration publish` — publish inspiration assets with category validation and automatic storage tiering (`omnimux-files` for users with 72h expiry, `omnimux-inspiration` for admins with permanent access).
- `omnimux inspiration get <share_id>` — retrieve published inspiration details directly by share id.
- Supports reading prompt payloads directly from local text or JSON files via `--prompt <file_path>`.

## 0.4.0

### Breaking changes

- Social accounts, connect, disconnect, post and post-status commands require `--provider zernio|tiktok_direct`. Missing or unknown providers fail locally with usage exit code 3 before any request. There is no default provider.
- GET/DELETE requests carry `provider` in the query; POST requests carry it in the JSON body. Cross-source account/post requests are rejected by the gateway with HTTP 409.
- Credentials now use `~/.config/omnimux/secrets.json` (mode `0600`) on every platform, with shared credential-slot support. Existing CLI-managed file credentials are preserved. Keychain-only users must sign in again or securely configure a credential; updating the CLI does not rotate a PAT.

### Included since 0.3.0

- User-level hosted channel, order and task commands with typed validation and expanded tests.
- Social seat summaries and official TikTok connect platform validation.
- Hosted-operation guidance and UGC/video skills with talking-head pacing and identity guidance.
- The Agent RPC envelope remains `contract_version: 1`.

### Upgrade and migration

```bash
omnimux update                         # binary installation
npm install -g @omnimux/cli@0.4.0      # npm installation
omnimux --version
omnimux social accounts --provider zernio
omnimux social accounts --provider tiktok_direct
omnimux social post-status <post-id> --provider zernio --no-refresh
```

Update saved scripts to pass the source explicitly for all five social operations. Upgrade alone cannot rewrite consumer scripts. Use `omnimux schema` for command contracts. The gateway began enforcing the provider requirement at **2026-09-07 07:25:20 UTC+8 (2026-09-06T23:25:20Z)**; this is separate from the CLI release date.

## 0.3.0

**Breaking: Agent RPC wire contract (`contract_version: 1`).**

### Breaking changes

- Success stdout is always `{ "ok": true, "data": ..., "meta": { "command", "cli_version", "contract_version" } }`. Former top-level fields live under **`data`**.
- Errors are JSON on **stderr** (`ok: false`, `error.type` / `subtype` / `recovery`), not plain text lines.
- Exit codes: `0` ok, `1` api, `2` config/auth, `3` usage, `4` network, `5` internal, `10` confirmation.
- Agents must branch on `ok` / `error.type` / exit code — never on free-text `message`.

### Added

- `omnimux schema [leaf]` — machine-readable command catalog
- Login split-flow: `login --no-wait`, `login --resume <flow_id>`, `login status [--verify]` (server `device_code` never printed)
- Risk gates: `--dry-run` (fail-closed when unsupported), `--yes` for high-risk writes (exit 10)
- `meta.notices` for update / skills drift (suppress with `OMNIMUX_NO_*_NOTIFIER`)
- `omnimux skill read <id>` — package `SKILL.md` (npm/source; binary without `skill/` → typed error)
- Normative docs: `docs/ERROR_CONTRACT.md`

### Migration

```bash
# Before 0.3.0
omnimux models | jq '.data'

# After 0.3.0
omnimux models | jq '.data'   # envelope; business payload under .data
# errors:
omnimux models 2>err.json; jq '.error.type' err.json
```

See `docs/ERROR_CONTRACT.md` and `skill/omnimux/SKILL.md`.

## 0.2.x

Prior user-level CLI releases (models, pricing, tokens, skills, social). See git tags `cli-v0.2.*`.
