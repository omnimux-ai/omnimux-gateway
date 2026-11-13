# OmniMux runtime binding — slideshow-creation

Methodology, modes, and platform packs stay in `SKILL.md` and `references/`. This file is host runtime only.

## When to load

User wants multi-page **图文** (Xiaohongshu / Douyin photo / TikTok Photo Mode style): outline, copy pack, reference breakdown, diagnosis, or explicit page-image generation.

Always load sibling skill **`omnimux`** for secrets and CLI.

## CLI prerequisites

```bash
omnimux config
omnimux models
omnimux pricing
omnimux balance
omnimux tokens
```

Use `omnimux tokens exec` with `__OMNIMUX_TOKEN_{id}__` only — never paste `sk-` into chat.

## Capability map

| User goal | Owner |
| --- | --- |
| Outline / copy / reference breakdown / diagnosis | This skill |
| List models / pricing / balance | `omnimux` CLI |
| Generate **page images** | `POST {base}/v1/images/generations` via `tokens exec` |
| Full carousel “template persist” | Out of scope for OmniMux CLI (deliver files/URLs + copy pack) |

## Optional image generation (after explicit user intent)

1. Finish methodology: page table + per-page visual direction + prompts.
2. Confirm paid generation with the user.
3. Pick an image model from `omnimux models` / `pricing`.
4. Call via `omnimux tokens exec` (see hub `docs/actions-exec.md`).

Shape only (host from `omnimux config`):

```bash
omnimux tokens exec TOKEN_ID --timeout=300 -- curl -sS \
  -X POST "https://api.example.com/v1/images/generations" \
  -H "Authorization: Bearer __OMNIMUX_TOKEN_TOKEN_ID__" \
  -H "Content-Type: application/json" \
  -d '{"model":"<image-model-id>","prompt":"<page prompt>","n":1}'
```

Generate one page at a time unless the model contract supports batches. Report each page’s URL or blocker.

## Social-media ops flow

1. Platform + audience + product constraints  
2. Mode from `SKILL.md` (outline vs copy vs generate)  
3. Platform deep packs under `references/platforms/` when platform is known  
4. Optional image generation via OmniMux  
5. Publish remains human / external tools  

## Hard rules

- No admin endpoints.  
- No claim of persisted host templates without evidence.  
- Non-generation modes must not force image API calls.  
- Prefer methodology completion over premature generation.
