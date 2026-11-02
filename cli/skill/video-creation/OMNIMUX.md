# OmniMux runtime binding — video-creation

Methodology stays in `SKILL.md` and support files. This file is host runtime only.

## Shared

Read sibling skill **`omnimux`** first (CLI, secrets, `tokens exec`). Never paste `sk-` into chat.

## CLI

```bash
omnimux models
omnimux pricing
omnimux balance
omnimux tokens
```

## Capability map

| Goal | Surface |
| --- | --- |
| Scripts / analysis / storyboard / prompt packs | This skill |
| Account / models / keys | `omnimux` |
| Video generation | `POST {base}/v1/video/generations` then poll `GET .../video/generations/{task_id}` via `tokens exec` |
| Image stills (optional) | `POST {base}/v1/images/generations` via `tokens exec` |

## Optional generation

Only after explicit user intent. Confirm paid / long-running runs. Pick model from `omnimux models`.

```bash
omnimux tokens exec TOKEN_ID --timeout=600 -- curl -sS \
  -X POST "https://api.example.com/v1/video/generations" \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  -H "Content-Type: application/json" \
  -d '{"model":"<video-model-id>","prompt":"<prompt>"}'
```

`task_id` alone is partial — poll until artifact URL or clear blocker.

## Long-running task UX

Talking-head / multi-take UGC often runs for minutes. Keep the user informed; do not go silent.

1. **Plan first** (after explicit generate intent): takes count, identity reuse vs new face, captions on/off, rough wall-time if known, model choice.
2. **Confirm** paid or long-running runs before `tokens exec`.
3. **Submit** → report `task_id` as **in progress**, never as finished video.
4. **Poll** `GET {base}/v1/video/generations/{task_id}`; narrate status changes in plain language (queued / running / succeeded / failed).
5. **Success evidence**: artifact URL fields such as `videoUrls` / `videoUrl` / equivalent — not `task_id` alone.
6. **Multi-take**: report each take, then summarize the set.
7. **Failure**: surface provider/API error text; map to user actions (quota/balance, invalid input, policy). Do not invent foreign host error codes.

## Hard rules

- No admin endpoints.
- Do not invent Gxgen/editor workflow tool names.
- Analysis/script/storyboard modes must not force generation.
- Captions stay off unless the user asked (see talking-head contract in `SKILL.md`).
