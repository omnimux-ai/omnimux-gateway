# cli/AGENTS.md — OmniMux user-level CLI conventions

Scope: everything under `cli/`. Root `AGENTS.md` also applies.

## Product boundary (user-level only)

- This package is the **end-user** CLI + skill. It must never grow admin or instance-operations capabilities (channels, inventory, health, smoke, user management, options, …).
- **No admin endpoints in source.** `test/no-admin.test.js` scans `src/` and fails if any route prefix guarded by `AdminAuth()`/`RootAuth()` in `router/api-router.go` (and `router/channel-router.go` / `router/authz-router.go`) appears as a string. Keep that blacklist in sync when upstream adds admin route groups.
- No developer-machine logic: no multi-profile registries, no multiple keychain backends, no CI branches, no `.env` file chains. The configuration model is exactly: flag > env > `~/.config/omnimux/config.json`, with secrets in `~/.config/omnimux/secrets.json` mode 0600 on every platform.
- Shared logic belongs in `cli/src/lib/` (the single source of truth); `commands/` are thin adapters; `skill/` teaches how to call the CLI and hosts methodology skill packs.
- **Skill layout:** flat `skill/<skillId>/` (shared `omnimux` + business skills); `manifest.json` lists installable ids. Agent surface is `.agents/skills/<id>/` via `omnimux skill install` (no nested packs).
- **No vendored install surface in this repo:** `skill/` is the only source for product skills. This monorepo's `.agents/skills/` holds development skills only (`newapi`, `deploy-vps`, `cli-release`, `fork-sync`, …). MUST NOT commit product skills under `.agents/skills/<id>`: that creates a second source that drifts. Install product skills on demand into a user project or `~/.agents/skills`; repo agents can read `cli/skill/<id>/SKILL.md` directly.
- **Pack migration:** prefer 100% original methodology text; change only brand names, exclusive host tools/workflows, and catalog paths that do not apply to OmniMux. Methodology packs stay `codeExecutionCore: false` unless explicitly classified otherwise. Full process + self-evolution: `.agents/skills/omnimux-cli-skill-migrate`.
- **Skill install for end users:** `omnimux skill install|update|status` installs **each** skill as a sibling under `.agents/skills/<id>` (schema 2 registry). Prefer `link`. Legacy 0.2.6 whole-tree installs migrate on reinstall. Successful `omnimux update` runs skill sync.

## Secret handling (hard rules)

1. A full `sk-` key must never appear on stdout, stderr, logs, or error messages. Defensive sanitization lives in `src/lib/mask.js` and `src/lib/output.js`.
2. The real token key moves only: API response → clipboard pipe / file placeholder / child process. Never through a variable that gets printed.
3. The access token is never written to `config.json` and never printed; `config get access_token` returns presence/length metadata only.
4. No backdoors, no debug output, no "temporary" logging of Authorization headers or request bodies in `src/lib/http.js`.
5. Placeholder format is strictly `__OMNIMUX_TOKEN_{id}__` for `tokens apply` / `tokens exec`.

## Output contract (Agent RPC)

Normative: `cli/docs/ERROR_CONTRACT.md` (`contract_version: 1`).

- Success stdout: `{ "ok": true, "data": ..., "meta": { "command", "cli_version", "contract_version" } }` — compact by default, `--pretty` indents.
- Error stderr: `{ "ok": false, "error": { "type", "subtype", "message", ... }, "meta": ... }`.
- Agents MUST branch on `ok` / `error.type` / `error.subtype` / exit code / `recovery` — never on `message` text.
- Exit codes: `0` ok, `1` api, `2` config/auth, `3` usage, `4` network, `5` internal, `10` confirmation.
- Typed errors: `cli/src/lib/errors.js` only for command-facing failures; serialize via `lib/output.js` allowlist (never `cause`/stack).
- `tokens exec` post-spawn exception: child stdout/stderr passthrough (sanitized) + child exit code; pre-spawn uses CLI JSON errors.
- Discoverability: `omnimux schema` / `omnimux schema <leaf>`.

## Code conventions

- Pure JavaScript, **CommonJS**, zero runtime dependencies, Node >= 18 and Bun dual-compatible. Only Node built-ins (`fetch`, `fs`, `child_process`, …). No TypeScript, no build step; binaries come from `bun build --compile`.
- Tests use `node:test` + `node:assert/strict` so they run under both `bun test` and `node --test`. Avoid `test(..., { skip: "reason" })` — bun's node:test shim ignores string skips; use conditional `test.skip` (see `test/contract.test.js`).
- Endpoint paths and security mechanisms are **ports** of `.agents/skills/newapi/scripts/` (api.js, http.js, sanitize.js, copy-key.js, inject-key.js, exec-token.js, fetch-key.js), simplified per the boundary rules above. When the dev skill's endpoints change, re-port deliberately.
- JSON responses for key-carrying paths (`/api/token`, `/api/user`) must go through `maskSensitiveJson`; text output through `sanitize`.

## Commands

```bash
cd cli
bun test                   # all tests (contract tests self-skip without OMNIMUX_TEST_BASE_URL)
bun src/bin.js help        # run from source
bun run build:binaries     # dist/ binaries: darwin-arm64, darwin-x64, linux-x64, windows-x64
```

## Distribution (One-Path)

### Repositories

| Role | Repository | Visibility | Contents |
|------|------------|------------|----------|
| **Source of truth** | this repository, path `cli/` | Public | Source, tests, `package.json` version, install script source |
| **Release surface** | `laozhong86/omnimux-cli` | **Public** | GitHub Release assets only (binaries, `SHA256SUMS`, `install.sh`), thin README |
| **npm** | `@omnimux/cli` | Public | Same version as `cli/package.json` |

- Agents MUST develop features only under monorepo `cli/`.
- Agents MUST NOT treat `omnimux-cli` as a second development mainline and MUST NOT mirror `cli/src` there for day-to-day work.
- User-facing install one-liner (when CDN is live): `curl -fsSL https://omnimux.ai/install.sh | bash`.
- Binary download target inside the script: `https://github.com/laozhong86/omnimux-cli/releases/...` (`OMNIMUX_RELEASE_REPO` in `scripts/install.sh`).

### Version and tags

- Single version number: `cli/package.json` → `version`.
- CLI git tags on the monorepo MUST be `cli-vX.Y.Z` (example: `cli-v0.2.0`).
- Agents MUST NOT use bare `vX.Y.Z` for CLI releases (reserved for gateway/app / Electron workflows).
- Pushing `cli-v*` MUST trigger `.github/workflows/cli-release.yml` only; gateway `.github/workflows/release.yml` MUST exclude `cli-v*`.

### Agent development flow (feature work)

1. Worktree + feature branch off latest `origin/main` (root AGENTS.md).
2. Keep user-level boundary; `bun test` before commit.
3. Prefer commit prefix `cli:` or `fork(cli):` for packaging/distribution changes.
4. Do not bump `package.json` version in unrelated gateway PRs.

### Agent release flow (human-gated)

**Trigger:** human explicitly requests a CLI release (e.g. "release CLI 0.3.0" / "执行 cli release").

1. `git fetch origin`; use a clean worktree; do not tag from a dirty primary checkout.
2. Ensure `cli/package.json` version equals the target semver; otherwise open a version-bump PR first.
3. Confirm `scripts/install.sh` has `OMNIMUX_RELEASE_REPO=laozhong86/omnimux-cli`.
4. Confirm monorepo secrets exist for CI: `OMNIMUX_CLI_RELEASE_TOKEN` (required), `NPM_TOKEN` (optional).
5. Optional local check: `cd cli && bun test && bun run build:binaries`.
6. Verify explicit release authorization covers the target version and publication channels. Reuse matching approval across turns; ask only for missing scope, changed risk, or an explicitly requested final review gate. Then:
   - `git tag cli-vX.Y.Z`
   - `git push origin cli-vX.Y.Z`
7. Monitor Actions: `CLI Release` succeeds; gateway `Release` workflow is **not** triggered.
8. Verify public assets and `omnimux --version` via install path.
9. If npm job fails: report dual-channel skew; do not force-publish without human order.
10. Agents MUST NOT run `npm publish` or `gh release create` locally unless explicit authorization covers that manual hotfix. Existing approval for the same hotfix persists across turns; a routine CI release request does not authorize a manual fallback.

Release checklist skill: `.agents/skills/cli-release/SKILL.md`.

### Hard prohibitions

- MUST NOT push to `upstream`.
- MUST NOT force-push published `main` or rewrite published `cli-v*` tags without human order.
- MUST NOT print `sk-` keys, `OMNIMUX_CLI_RELEASE_TOKEN`, or `NPM_TOKEN`.
- MUST NOT claim an installation path works while its installer or release assets are unavailable. The current character landing has no quick-start/download section; do not restore retired install UI from this CLI runbook.
- MUST NOT implement admin routes in this package.

### Incidents

- Bad binary: ship the next patch tag; destructive Release edits need human confirmation.
- Wrong tag fires gateway build: cancel the run; fix tag filters; document in ops notes.
- Broken install script: fix monorepo `cli/scripts/install.sh`, re-release; purge CDN cache if used.

## Attribution (protected)

README/license references to **new-api** and **QuantumNous** are protected project information per root `AGENTS.md` — never remove or rewrite them.
