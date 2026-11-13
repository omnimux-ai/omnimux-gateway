# OmniMux runtime binding — recreate-viral-video

This file is **host runtime only**. Methodology, evidence grades, compliance, and output contracts remain in `SKILL.md` and `docs/`.

Product fit: social-media operations — turn reference / viral structure into an original plan for the user's product or topic, then optionally generate media through OmniMux APIs.

## When to load this pack

1. User wants reference-video **safe adaptation** (structure transfer, not 1:1 copy).
2. Read pack root `SKILL.md`, then `docs/usage.md` as needed.
3. Use hub skill `omnimux` (`cli/skill/SKILL.md`) for install, auth, models, pricing, and secret-safe token use.

Do **not** claim you watched a URL-only reference. Do **not** skip fit-grade and risk sections when the user asks for a full recreation plan.

## OmniMux CLI prerequisites

```bash
omnimux config            # secret-safe status
omnimux models            # discover available models on the instance
omnimux pricing           # public pricing when planning cost
omnimux balance           # optional: check quota before paid generation
omnimux tokens            # list user API tokens (keys masked)
```

All mutating API calls that need an `sk-` key must go through `omnimux tokens exec` with placeholder `__OMNIMUX_TOKEN_{id}__`. Never paste real keys into chat, files, or skill prompts. Details: hub `docs/actions-exec.md`.

## Capability map (methodology vs execution)

| User goal | Owner | Surface |
| --- | --- | --- |
| Structure breakdown, fit grade A–D, original script / storyboard / prompt pack | This pack | `SKILL.md` + docs/prompts/schemas |
| List models / pricing / balance | Hub CLI | `omnimux models` / `pricing` / `balance` |
| Submit **image** generation | Hub CLI + OpenAI-compatible API | `POST {base}/v1/images/generations` via `tokens exec` |
| Submit **video** generation (async task) | Hub CLI + OpenAI-compatible API | `POST {base}/v1/video/generations` then `GET {base}/v1/video/generations/{task_id}` via `tokens exec` |

This pack does **not** own billing, channel routing, or provider selection. Pick models that exist on the user's OmniMux instance (`omnimux models` / `pricing`).

## Generation handoff (optional)

Only after the user explicitly wants media output (not analysis-only):

1. Finish methodology output (at least fit grade + original prompt / storyboard + negative constraints).
2. Resolve `base_url` from `omnimux config` (default production instance when unset).
3. Choose a video (or image) model id the instance actually exposes.
4. Confirm paid / long-running generation with the user before calling APIs.
5. Call APIs via `omnimux tokens exec` so the key never enters the conversation.

### Example: text-to-image (shape only)

```bash
# TOKEN_ID from `omnimux tokens` (never put a real sk- here)
omnimux tokens exec TOKEN_ID --timeout=300 -- curl -sS \
  -X POST "https://api.example.com/v1/images/generations" \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  -H "Content-Type: application/json" \
  -d '{"model":"<image-model-id>","prompt":"<from final_video_prompt or image brief>","n":1}'
```

Replace the host with the user's configured OmniMux API base (often the same host as console, or `api.` on production). Use `omnimux config` / hub setup docs for the live base URL.

### Example: video generation + poll (shape only)

```bash
omnimux tokens exec TOKEN_ID --timeout=600 -- curl -sS \
  -X POST "https://api.example.com/v1/video/generations" \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  -H "Content-Type: application/json" \
  -d '{"model":"<video-model-id>","prompt":"<from generation_handoff.final_prompt>"}'

# Then poll with the returned task id (field names vary by channel; use the JSON the API returns):
omnimux tokens exec TOKEN_ID --timeout=60 -- curl -sS \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  "https://api.example.com/v1/video/generations/<task_id>"
```

If the chosen model requires image/video references, pass only **user-provided or authorized** media URLs / base64 per that model's contract. Never download or re-host copyrighted full videos as "reference" without user rights.

## Social-media ops suggested flow

1. **Brief** — product, audience, platform, must-not-say constraints.
2. **Evidence** — classify reference evidence grade (see `SKILL.md`).
3. **Adapt** — this pack: structure → fit grade → original plan / prompt pack.
4. **Generate** (optional) — OmniMux image/video APIs via `tokens exec`.
5. **Publish** — out of scope for this pack (human or later publish skills).

## Hard rules

- Do not invent OmniMux admin or channel endpoints; this stack is **user-level** only.
- Do not hardcode a single upstream vendor as required for success.
- Do not treat a returned `task_id` alone as success when the user asked for a finished video; report status and artifact URL when available, else partial + next poll step.
- Prefer methodology completion over premature generation when inputs are weak.
