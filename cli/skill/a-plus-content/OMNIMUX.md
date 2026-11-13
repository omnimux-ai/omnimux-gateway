# OmniMux runtime binding — a-plus-content

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
| Planning / prompts / diagnosis | This skill |
| Account / models / keys | `omnimux` |
| Image generation | `POST {base}/v1/images/generations` via `omnimux tokens exec` |

## Optional generation

Only after explicit user intent. Confirm paid runs. Use a model from `omnimux models`.

```bash
omnimux tokens exec TOKEN_ID --timeout=300 -- curl -sS \
  -X POST "https://api.example.com/v1/images/generations" \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  -H "Content-Type: application/json" \
  -d '{"model":"<image-model-id>","prompt":"<prompt>","n":1}'
```

## Hard rules

- No admin endpoints.
- Do not invent host-only editor/workflow tools.
- Methodology completion is valid without generation.
