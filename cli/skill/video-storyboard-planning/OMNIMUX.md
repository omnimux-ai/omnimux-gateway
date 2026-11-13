# OmniMux runtime binding — video-storyboard-planning

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

## Hard rules

- No admin endpoints.
- Do not invent Gxgen/editor workflow tool names.
- Analysis/script/storyboard modes must not force generation.
