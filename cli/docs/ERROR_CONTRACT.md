# OmniMux CLI Error & Output Contract

**contract_version:** `1`  
**Audience:** AI Agents, shell scripts, CLI maintainers.

This is the wire contract for `@omnimux/cli` / `omnimux`. Branch only on stable fields below — never on `message` wording.

## Success (stdout, exit 0)

```json
{
  "ok": true,
  "data": {},
  "meta": {
    "command": "models",
    "cli_version": "0.2.11",
    "contract_version": 1
  }
}
```

- Success responses write **only** to stdout. stderr must stay empty for agent-parseable paths.
- Former top-level payload fields live under **`data`**.
- Optional `meta.notices` may carry update/skills drift hints:

```json
"meta": {
  "notices": {
    "update": {
      "current": "0.2.11",
      "latest": "0.3.0",
      "message": "…",
      "recovery": { "command": "omnimux", "args": ["update", "--yes"] }
    },
    "skills": {
      "drift": true,
      "count": 2,
      "recovery": { "command": "omnimux", "args": ["skill", "update"] }
    }
  }
}
```

Suppress: `OMNIMUX_NO_UPDATE_NOTIFIER=1`, `OMNIMUX_NO_SKILLS_NOTIFIER=1`.

## Error (stderr, non-zero exit)

```json
{
  "ok": false,
  "error": {
    "type": "auth",
    "subtype": "token_missing",
    "message": "access token is not configured",
    "hint": "sign in or set a token, then retry",
    "retryable": false,
    "recovery": {
      "command": "omnimux",
      "args": ["login", "--no-wait"]
    }
  },
  "meta": {
    "command": "models",
    "cli_version": "0.2.11",
    "contract_version": 1
  }
}
```

### Allowlisted `error` fields

| Field | Stability | Notes |
|-------|-----------|-------|
| `type` | wire-stable | `usage` \| `config` \| `auth` \| `api` \| `network` \| `internal` \| `confirmation` |
| `subtype` | wire-stable | declared constants |
| `message` | informational | **do not branch** |
| `hint` | informational | human guidance |
| `retryable` | wire-stable | boolean |
| `param` | per-subtype | flag/arg name when applicable |
| `recovery` | wire-stable when present | CLI-owned `{ command, args }` |
| `request_id` | when known | HTTP tracing correlation, never the client creation identity |
| `operation_id` | creation/recovery | Client creation request ID; preserve for reconciliation |
| `task_id` | creation/wait/download | Original generation task; preserve instead of resubmitting |
| `action` / `risk` | confirmation | high-risk gate |
| `status` | when known | HTTP status |

**Never serialized:** `cause`, stack traces, headers, raw response bodies, tokens.

### Exit codes

| type | exit |
|------|------|
| (success) | 0 |
| api | 1 |
| config, auth | 2 |
| usage | 3 |
| network | 4 |
| internal | 5 |
| confirmation | 10 |

## Agent decision table

| `error.type` | Default agent action |
|--------------|----------------------|
| `usage` | Fix argv; run `omnimux schema` — no retry |
| `config` | Apply `recovery` or config set |
| `auth` | `login --no-wait` then `login --resume <flow_id>` |
| `api` | Branch subtype; respect `retryable` |
| `network` | Backoff retry |
| `internal` | Report bug; do not blind-retry |
| `confirmation` | Verify existing explicit approval covers this action/risk; ask only if missing, then re-run with `--yes` |

## Login split-flow (Agent)

1. `omnimux login --no-wait` → `data.verification_url`, `data.user_code`, `data.flow_id` (never `device_code`)
2. End turn; user opens URL
3. After user confirms: `omnimux login --resume <flow_id>`
4. `omnimux login status [--verify]` for machine status

## Side effects: dry-run and confirmation

| Flag | Behavior |
|------|----------|
| `--dry-run` | No side effects when supported; success `data.dry_run: true` + preview. Unsupported leaves → `usage/dry_run_unsupported` (fail closed). Suppresses background auto-update. |
| `--yes` | Required for `high-risk-write` leaves (`tokens.apply`, `tokens.exec`, `update` install, `influencer.character.create`, `influencer.character.multiview`, `influencer.video.create`). Without it → exit **10** `confirmation/confirmation_required`. |

Supported dry-run leaves (see `omnimux schema`): `tokens.create`, `tokens.switch-group`, `tokens.apply`, `update` (check-only), `influencer.character.create`, `influencer.character.multiview`, `influencer.video.create`.

Agents **must not** append `--yes` without explicit human approval covering the target action and risk. Matching approval persists across turns; a confirmation error does not itself require a second approval. New side effects or an explicitly reserved final approval step require confirmation. Human-only payments remain human-only.

## Exception: `tokens exec` (two-phase)

| Phase | Output | Exit |
|-------|--------|------|
| Pre-spawn (bad args, missing token, spawn fail) | stderr JSON envelope | CLI taxonomy |
| Post-spawn (child started) | Child stdout/stderr passthrough (sanitized) | **Exact child status** |

Child exit `10` is **not** OmniMux confirmation.

## Operation failures

Do not return `ok: true` with `data.success: false`. Failed operations must use the error envelope.

## Discoverability

```bash
omnimux schema
omnimux schema tokens.create
omnimux help   # data.usage + data.commands
omnimux skill list
omnimux skill read omnimux   # package SKILL.md (npm/source; binary without skill/ → typed error)
```

## Source

- Types: `cli/src/lib/errors.js`
- Serialization: `cli/src/lib/output.js`
- HTTP map: `cli/src/lib/http.js`

## Character/video creation recovery

Creation success means durably accepted, not delivered. `influencer task get` may successfully return a failed task; `influencer task wait` succeeds only for a succeeded task with a ready artifact. Wait defaults to 600 seconds. `api/wait_timeout` (exit 1) preserves `task_id`, and never cancels/resubmits a cloud task.

| Subtype | Required action |
|---|---|
| `creation_not_supported` (api) | The configured service lacks this capability; use a compatible service version |
| `wait_interrupted` (api) | Resume waiting for the original task; cloud generation continues |
| `scope_required` (auth) | Explicitly authorize the new creation permissions with login; old credentials never expand automatically |
| `influencer_creation_disabled` (api) | Read/recovery remain available; wait for service enablement, do not use another identity |
| `creation_route_unavailable`, `generation_route_unavailable`, `reference_route_unavailable` (api) | No compatible creation route is currently available; non-retryable. Keep the original request identity and recheck service availability before creating. |
| `creation_request_conflict` (api) | Same client ID has different intent/material; inspect the original request, do not silently replace it |
| `creation_request_expired` (api) | Recovery lifetime ended; that ID can never be reused for another bill |
| `receipt_unavailable` (config) | Restore private record storage; lookup the original operation before any uncertain retry |
| `creation_request_in_progress` (api) | Another local process owns this ID; wait and read the request |
| `generation_failed` (api) | Read the original task and known quota/refund facts; no automatic new generation |
| `artifact_unavailable` / `artifact_invalid` / `artifact_expired` (api) | Re-read/download the original artifact; no new generation |
| `output_exists` (usage) | Select another output path or explicitly authorize overwrite |
| `download_interrupted` (network) | Retry only the original task download |

Creation errors expose `operation_id` separately from HTTP `request_id`. Upload references and the exact final POST body are saved privately before submission. Any attempted POST is followed by request lookup on a later invocation; only a definitive not-found response allows retry of the original body and same ID. Never infer non-acceptance from timeout, connection loss, or a service error.

Creation dry-run is offline, performs no secret lookup/upload/journal/update, and marks server media inspection pending. Download authenticates the metadata API, uses capability-signed media destinations without account credentials, refuses unsafe redirects and default overwrite, and reports success only after a nonempty validated file is atomically completed. No signed download URLs appear in download output or local receipts.

Artifact capabilities are private credentials. Ordinary influencer outputs recursively retain only artifact `key`, `type` and `mime_type`; they do not expose content/download URLs, including nested multiview and recovered-task results. Signed media URLs are also redacted from free text and early confirmation recovery arguments. Public unsigned option imagery remains available. Use `influencer task download` to save an artifact; its internal URL is never printed or journaled.

Download requires a complete HTTP 200 response without `Content-Range`; HTTP 206, other non-complete success responses and any explicit partial-range response produce `api/artifact_invalid` before writing media. A valid container prefix or a length matching only one fragment cannot establish successful download.
