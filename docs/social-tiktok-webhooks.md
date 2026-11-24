# TikTok Content Posting Webhooks

This document describes the gateway receiver for official TikTok Content Posting API webhooks (`POST /api/social/v1/webhooks/tiktok`), its security controls, and its monotonic state transition contract.

## Endpoint & Fail-Closed Configuration

- **Route**: `POST /api/social/v1/webhooks/tiktok` (public route; exempt from `UserAuth` and `CriticalRateLimit`, inherits `GlobalAPIRateLimit` and `AnonymousRequestBodyLimit`).
- **Response headers**: every response sets `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`.
- **Body limit**: the handler enforces a strict `64 KiB` raw payload cap using `io.LimitReader(body, 64<<10+1)`. Oversized requests immediately return `413 Payload Too Large` without truncating into signature verification.
- **Default-off (fail-closed)**:
  - `TIKTOK_WEBHOOK_ENABLED` (default `false`): must be `true` together with `SOCIAL_OPS_ENABLED=true`, `TIKTOK_DIRECT_ENABLED=true`, `ROLLBACK_TIKTOK_TO_ZERNIO=false`, and active TikTok app credentials (`TIKTOK_CLIENT_KEY` + `TIKTOK_CLIENT_SECRET`, or `TIKTOK_SANDBOX_CLIENT_KEY` + `TIKTOK_SANDBOX_CLIENT_SECRET` when `TIKTOK_SANDBOX_ENABLED=true`).
  - If any gate or active credential is missing, the handler returns `403 Forbidden` **before reading the request body**.
  - `TIKTOK_WEBHOOK_TOLERANCE_SECONDS` (default `300`): replay tolerance window in seconds for the `TikTok-Signature` timestamp `t`, clamped to `[30, 3600]`.
- **Status probe (`GET /api/social/v1/status`)**:
  - Top-level `webhook_configured` remains unchanged for legacy Zernio compatibility.
  - `tiktok_direct` exposes two boolean readiness probes without leaking any secret material:
    - `webhook_enabled`: reflects `TIKTOK_WEBHOOK_ENABLED`
    - `webhook_configured`: reflects whether active `TikTokCredentials()` (`client_key` and `client_secret`) are non-empty

## Signature Verification & Replay Protection

Per TikTok's official webhook verification specification, TikTok signs deliveries with the app's `client_secret` using HMAC-SHA256:

- **Header**: single `TikTok-Signature: t=<unix_epoch_seconds>,s=<hex_hmac_sha256>`
- **Signed message**: `<t>.<raw_request_body>` (exact raw bytes; never re-serialized JSON)

Verification executes in strict order:

1. Require exactly one non-empty `TikTok-Signature` header containing exactly one `t` (positive ASCII decimal integer) and one `s` (64-character hex SHA-256 MAC), with no duplicate or unknown parameters.
2. Compute HMAC-SHA256 over `<t>.<raw_body>` with the active `client_secret` and compare in constant time (`hmac.Equal`).
3. Validate the timestamp window: `past <= TIKTOK_WEBHOOK_TOLERANCE_SECONDS` and `future <= 60s`.
4. Parse the raw body as a JSON object envelope (`client_key`, `event`, `create_time`, `user_openid`, `content`). Invalid JSON under a valid signature returns `400 Bad Request`.
5. Compare top-level `client_key` against the active app `client_key` in constant time (`subtle.ConstantTimeCompare`).

All five authentication failures return an identical generic `401 Unauthorized` response body to prevent oracle leakage, while server-side `WARN` logs (localized via `common.LogText`) distinguish:

- `missing_header`
- `malformed_signature`
- `bad_hmac`
- `timestamp_out_of_window` (includes safe numeric `sig_age_seconds` and `tolerance_seconds`)
- `client_key_mismatch`

No log line or error response ever includes `client_secret`, the raw `TikTok-Signature` header value, or raw request body bytes.

## Supported Events & Monotonic State Machine

Supported `post.publish.*` events (with `content` accepted as TikTok's serialized JSON string or a JSON object, and `post_id` accepted as either JSON string or 64-bit `json.Number`):

| Event | Allowed Transitions & Snapshot Merge Behavior |
| --- | --- |
| `post.publish.inbox_delivered` | `submitted`/`processing` → `sent_to_inbox`; idempotently updates `sent_to_inbox` snapshot; ignored on `published`/`failed`/`partial`/`deleted`. |
| `post.publish.complete` | `submitted`/`processing`/`sent_to_inbox` → `published`; on already-`published` rows merges metadata (`publish_complete: true`) while preserving existing `post_ids`/`unavailable_post_ids`; ignored on `failed`/`partial`/`deleted`. |
| `post.publish.publicly_available` | `submitted`/`processing`/`sent_to_inbox` → `published` and records `post_id` in `result_snapshot.post_ids` (deduplicated); on already-`published` rows deduplicates and appends `post_id`; ignored on `failed`/`partial`/`deleted`. |
| `post.publish.failed` | `submitted`/`processing`/`sent_to_inbox` → `failed` (records `reason` and `fail_reason`); never overwrites `published`/`failed`/`partial`/`deleted`. |
| `post.publish.no_longer_publicaly_available` (and defensive spelling `post.publish.no_longer_publicly_available`) | Appends `post_id` (when present) to `result_snapshot.unavailable_post_ids`. In multi-post upload mode, if other known `post_ids` remain outside `unavailable_post_ids`, keeps `status = "published"` and updates the snapshot; once no surviving `post_ids` remain (single post, last surviving post, or no fine-grained `post_ids`), transitions one-way to `deleted` (`submitted`/`processing`/`sent_to_inbox`/`published` → `deleted`). Ignored on `failed`/`partial`/`deleted`. |

### Concurrency, Polling Co-Governance & Absorbing Terminals

- `deleted` is the final absorbing state and can never transition back to `published`, `sent_to_inbox`, `processing`, `submitted`, or `failed`. `failed` and `partial` are likewise irreversible terminal states.
- Both webhook deliveries and `GET /api/social/v1/posts/:id?refresh=1` polling (`refreshTikTokPostStatus`) share transactional row-locked updates in `model/social_post.go` via `lockForUpdate(tx)` (SQLite/MySQL/PostgreSQL compatible).
- Polling may refresh non-terminal sub-states (e.g. `PROCESSING_UPLOAD` → `PROCESSING_DOWNLOAD`), but never regresses `sent_to_inbox` back to `processing`, never overwrites terminal states, and never clears webhook-recorded `post_ids`.
- No-op replays skip `UPDATE` cleanly; database `RowsAffected == 0` on identical values is never treated as an error.
- Verified deliveries for non-target events (including probe events), unknown `publish_id`, or `user_openid` mismatches against the locally bound account (when both are non-empty and differ in constant time) do not modify the database and return `200 OK` to avoid 72-hour retry storms.

## HTTP Status Code Contract

| Status | Meaning |
| --- | --- |
| `200 OK` | Verified delivery handled (updated, idempotent no-op, ignored non-target event, unknown `publish_id`, or `user_openid` mismatch). |
| `400 Bad Request` | Body read error, or malformed JSON envelope / `content` under a valid signature. |
| `401 Unauthorized` | Missing/malformed `TikTok-Signature`, bad HMAC, timestamp outside replay window, or `client_key` mismatch. |
| `403 Forbidden` | Fail-closed: webhook disabled or active TikTok credentials unavailable (returned before reading body). |
| `413 Payload Too Large` | Request body exceeds the `64 KiB` handler limit (or route `AnonymousRequestBodyLimit`). |
| `500 Internal Server Error` | Transient database error during lookup or transactional update (triggers TikTok retry). |

## External Enablement Checklist (Outside Code)

Before enabling `TIKTOK_WEBHOOK_ENABLED=true` in a deployment:

1. Configure the public HTTPS callback URL `https://<your-gateway-host>/api/social/v1/webhooks/tiktok` in the TikTok Developer Portal under Webhooks.
2. Verify edge/WAF rules allow `POST /api/social/v1/webhooks/tiktok` with the `TikTok-Signature` header and do not rewrite the raw JSON body.
3. Run the portal `Test URL` check and end-to-end Sandbox publish event tests (`inbox_delivered`, `complete`, `publicly_available`, `failed`, `no_longer_publicaly_available`) and inspect structured `WARN` logs (including `sig_age_seconds` on retries).
4. Submit the Live App revision in TikTok Developer Portal if required for Webhook subscription changes, then enable `TIKTOK_WEBHOOK_ENABLED=true`.
