---
name: slideshow-creation
description: >-
  图文内容创作导演：用于小红书、抖音图文、TikTok Photo Mode、种草笔记、多页图文、
  文案包、参考拆解、图文诊断与正式生成。根据用户目标先判断交付模式，再决定是否生成或持久化。
  不适合纯图片、纯视频、仅保存链接、已有模板编辑、只要营销策略或不要求图文成品的脚本。
metadata:
  version: "9.0.0"
  author: omnimux
  tags: [slideshow, content-generation, social-media]
  category: content-generation
  publicationStatus: public
  codeExecutionCore: false
  compatibility: OmniMux CLI flat skill under cli/skill/slideshow-creation/. Pair with sibling skill `omnimux` for models/tokens/API execution.
---

# 图文内容创作导演

## OmniMux shared

开始前阅读 sibling skill **`omnimux`**（CLI 安装、密钥、`tokens exec`）。不要在对话中暴露 `sk-`。可选图片/图文页生成见 [`OMNIMUX.md`](OMNIMUX.md)。

This skill is product-semantic context, not a hidden route, fixed tool sequence, or completion gate. It helps the Agent turn a brief, product, reference, campaign idea, or platform goal into a useful slideshow-style content artifact.

The Agent decides whether to clarify, draft, analyze, generate, call tools, stop, or report a blocker. Tool schemas, persistence tools, billing, permissions, and runtime state own hard constraints and external side effects.

## Publication Governance

This skill is a public business-methodology skill, not a code-execution skill. The publication readiness record lives in [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md).

Do not turn slideshow planning into a fixed persistence workflow, required-next tool, sticky skill state, or template completion gate. Persisted templates require explicit user intent and real tool evidence.

## First Decision

Classify the current request by deliverable before choosing tools:

| Mode | Use when | Standard deliverable | Requires persisted artifact? |
| --- | --- | --- | --- |
| `brief-to-outline` | User gives a topic, product, audience, campaign, or rough idea and wants a plan. | A page-by-page content outline with hook, page purpose, visual direction, and CTA. | No |
| `copy-pack` | User wants text, page copy, captions, hashtags, titles, or ready-to-paste content. | Page copy pack plus visual hints and posting notes. | No |
| `reference-breakdown` | User provides a link, inspiration, screenshots, or a known style and asks to learn from it. | Reference breakdown with reusable structure, pacing, proof pattern, and do-not-copy boundaries. | No |
| `generate-slideshow` | User explicitly asks to create, generate, land, or produce a multi-page image-text template. | Persisted slideshow/template result or clear generation blocker. | Yes |
| `template-guided` | User selected or asks to adapt an inspiration/template into a new slideshow. | Persisted template generated from the selected structure, or clear blocker. | Yes |
| `variant/revision` | User asks for another version, shorter/longer pages, different tone, platform, product angle, or audience. | Revised outline/copy pack, or a persisted regenerated template if generation was explicitly requested. | Conditional |
| `slideshow-diagnosis` | User asks whether a note, outline, carousel, reference, or generated result works. | Diagnosis with page-level fixes and production recommendations. | No |

Do not convert a non-generation request into a persisted template task just because this skill is attached.

## Clarification Rule

Ask one high-value question when the missing answer would change:

- target platform
- page count
- aspect ratio or template format
- image source mode
- product proof or reference dependency
- generation cost or external side effect
- success standard

If the user explicitly says "直接生成", "先出一版", "不用问", or equivalent, proceed with stated assumptions and make those assumptions visible in the response.

Do not invent hidden defaults for platform, slide count, image source, product proof, or target audience when those choices would change the output.

## Creative Spine

For every mode, make the slideshow work as sequential persuasion:

1. Attention: why page 1 earns a stop.
2. Clarity: what the viewer understands within the first two pages.
3. Proof: what product, experience, comparison, data, or scenario makes the claim believable.
4. Rhythm: how each page changes the viewer's state instead of repeating the same point.
5. Visual execution: what the page should show, not only what it should say.
6. Action: what save, click, comment, buy, follow, or next step the content creates.

## Output Contracts

### `brief-to-outline`

Return a compact structure:

- goal and audience
- platform assumption
- page count assumption
- page table: page number, hook or headline, page job, body idea, visual direction, proof element
- CTA or posting recommendation
- risks or open questions

### `copy-pack`

Return copy that can be used by a human or downstream generator:

- title options
- page-by-page copy
- visual hints per page
- caption
- hashtags or search keywords when platform-relevant
- optional image-generation prompt notes

Do not claim that a template, image, or persisted result exists unless a tool produced it.

### `reference-breakdown`

Separate observation from adaptation:

- what the reference is doing
- page structure and pacing
- hook pattern
- proof and emotional levers
- visual system
- what to reuse abstractly
- what not to copy directly
- adaptation plan for the user's goal

### `generate-slideshow` / `template-guided`

Use generation only when the user has asked for a real multi-page slideshow/template result (not outline-only). On OmniMux, formal page-image generation is optional and goes through the **`omnimux`** skill + OpenAI-compatible image APIs (see [`OMNIMUX.md`](OMNIMUX.md)). There is no host “persist slideshow template” workflow in OmniMux CLI.

Generation planning inputs normally include:

- `prompt` / per-page image prompts
- `targetPlatform`
- `slideType` / narrative pattern
- `slideCount`
- `aspectRatio` when format-sensitive
- `imageSourceMode` (user assets vs AI-generated vs mix)
- inspiration or reference when explicitly selected
- product or audience context when provided

Useful execution surfaces (optional, never required by skill attachment alone):

- Sibling skill **`omnimux`**: `models` / `pricing` / `tokens exec` for image generation
- User-provided assets: local files or public URLs the user authorized
- Web research tools available in the host Agent (if any), only when the user needs fresh facts

Do not invent Gxgen editor/workflow tool names or claim a platform template was persisted unless a real tool returned an artifact.

### `variant/revision`

Respect the user's requested delta:

- shorter or longer
- different platform
- stronger product proof
- different tone
- different audience
- alternate visual system
- regenerated persisted template only if explicitly requested

### `slideshow-diagnosis`

Diagnose at page level:

- first-page stop power
- sequence clarity
- repeated or missing page jobs
- weak proof
- visual feasibility
- platform mismatch
- concrete fixes

## Tool And Runtime Boundaries

- Do not turn skill attachment into a fixed tool sequence, active tool expansion, `toolChoice`, hidden route, or completion gate.
- Do not call generation or persistence tools for `brief-to-outline`, `copy-pack`, `reference-breakdown`, or `slideshow-diagnosis` unless the user explicitly asks to generate.
- Do not treat link import, inspiration matching, or template selection as generation by itself.
- Do not claim a persisted slideshow/template unless a persistence tool returns a real template/result reference.
- Do not treat pure image creation, pure video creation, existing template editing, or strategy-only consulting as this skill.

## Completion Evidence

### General evidence

For non-generation modes, a complete answer can be:

- a usable outline
- a copy pack
- a reference breakdown
- a diagnosis with fixes
- a prompt-ready page plan
- a clear blocker with observed evidence and next step

No persisted artifact is required for these modes.

### Generation extension evidence

For `generate-slideshow` and `template-guided`, completion requires real generation evidence:

- image/API tool returned usable page images or URLs, and the response lists them page-by-page, or
- the called tool/API returned a clear blocker that the response reports.

An outline, page copy, image prompts, or "ready to generate" text is not final generation evidence. Do not claim a host template DB id unless one was actually returned.

## Reference Map (Low Freedom)

Read references only when the current mode needs them:

| 文件 | 何时读取 |
|------|----------|
| `references/requirements-fields.md` | 字段定义不清时 |
| `references/visual-presets.md` | 需要默认视觉方向时 |
| `references/design-principles.md` | 校正文案与版式原则时 |
| `references/plan-progress-patterns.md` | 恢复任务进展时 |
| `references/inspiration-candidate-display.md` | 灵感候选卡片展示语义 |
| `references/platforms/<platform>.md` | 目标平台已明确后 |
| `references/platform-deep-packs.md` | 平台总览要求下钻时 |

## Verification

- [ ] The response mode is clear from the user's request and the delivered artifact.
- [ ] Assumptions are stated when the user asked to proceed without clarification.
- [ ] Non-generation modes do not require persisted artifacts.
- [ ] Generation modes do not claim success without a real persisted result or clear tool blocker.
- [ ] Platform, page count, image source, and success criteria are not hidden magic defaults when they materially change the result.
- [ ] References and inspiration are abstracted into reusable structure, not copied blindly.
