---
name: video-creation
description: >-
  视频成片创作导演：当用户要求直接生成视频、继续出片、图生视频、参考视频生成、
  或已分析参考并明确要落地成片时激活。也可用于生成前的 brief、视频生成方案、
  prompt pack、模型选择建议、brief-first 澄清和成片诊断。不激活：仅剪辑已有视频、只写脚本/分镜表、
  只分析视频不出片、纯图片创作、图文创作，或只做模板匹配不要求生成。
metadata:
  publicationStatus: public
  codeExecutionCore: false
  compatibility: OmniMux CLI flat skill under cli/skill/video-creation/. Pair with sibling skill `omnimux`.
---

# 视频成片创作导演

## OmniMux shared

开始前阅读 sibling skill **`omnimux`**（CLI 安装、密钥、`tokens exec`）。不要在对话中暴露 `sk-`。可选生成见 [`OMNIMUX.md`](OMNIMUX.md)。



## Positioning

Use this skill to help the Agent turn a video goal, image, reference video, or template into an executable video deliverable plan. The skill provides product semantics, creative judgment, prompt quality rules, model-selection cues, and completion evidence rules.

This skill is context, not a workflow owner. The Agent decides whether to clarify, draft, analyze, plan, select a model, call tools, stop, or report a blocker. Tool schemas, provider adapters, billing, persistence, and atomic workflow tools own hard constraints and external side effects.

按状态门推进：先看当前 turn 已经具备什么证据，再决定澄清、写计划、给出模型偏好建议、调用 `generate-video`，或停止并报告 blocker。不要把本 skill 变成固定 workflow；不要幻想或调用 list/select/init/submit/wait 半流程工具。

模型专属 guidance 在已知目标模型/scene option，或准备把模型偏好传给 `generate-video` 时读取。Seedance 2.0 的 scene option 是 `seedance-2`；读取 `video-model-${sceneOptionId}`（即 `video-model-seedance-2`）来获得素材数量、画幅、清晰度和参考角色约束。

## Publication Governance

This skill is a public business-methodology skill, not a code-execution skill. The publication readiness record lives in [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md).

Do not turn video creation guidance into a fixed provider workflow, required-next tool, sticky skill state, or generation completion gate. Real video delivery requires explicit user intent, current model/tool evidence, and an actual returned artifact or clear blocker.

## Task Modes

Choose the mode by requested deliverable, not by keywords alone.

| Mode | Use When | Standard Deliverable | Requires video artifact |
| --- | --- | --- | --- |
| `brief-to-video-plan` | User wants a video direction, production plan, or generation-ready strategy | Brief summary, concept, audience, format, asset plan, model/tool path, risks | No |
| `prompt-only` | User wants a video generation prompt, prompt pack, or model-specific prompt rewrite | Final prompt, negative/avoid notes, parameters, asset-role notes | No |
| `reference-analysis` | User asks to analyze a video, image, template, or inspiration without asking for a new video | Reference breakdown, transferable structure, risks, optional generation notes | No |
| `generate-video` | User explicitly asks to generate, continue output, or return a video result | Real video task/result or clear blocker | Yes |
| `image-to-video` | User provides image(s) and asks to animate or make a video | Real video task/result or clear blocker | Yes |
| `reference-to-video` | User asks to adapt a reference video/template into a new generated video | Real video task/result or clear blocker | Yes |
| `variant/revision` | User asks to revise a prior plan, prompt, or generated video | Revised plan/prompt, or new video only if regeneration is requested | Depends |
| `video-diagnosis` | User asks why a video plan, prompt, generation result, or failure is weak | Diagnosis, fix list, next-step recommendation | No |

If the user asks for both analysis and generation, do the analysis only as much as needed to support the generation decision. If the user only asks for analysis, prompt writing, strategy, or diagnosis, do not require a video artifact.

## Clarification And Defaults

视频创作默认先走 brief-first。模板来源解析、模型选择和外部生成都必须在请求 Actionable Enough 后再推进；它们服务当前目标，不替代目标判断。

**Material Uncertainty**: missing information materially changes platform, aspect ratio, duration, model/input mode, cost, asset usage, external side effects, or success criteria.

Ask one high-value question when missing information would materially change platform, aspect ratio, duration, model/input mode, cost, asset usage, external side effects, or success criteria.

Proceed with stated assumptions only when the user clearly says to generate directly, continue output, make a first version, or use your judgment. State the working assumptions briefly before tool calls or final handoff.

Tool schema defaults such as `16:9`, `720p`, or `generateAudio=true` are execution fallbacks, not user-confirmed creative decisions. Do not present them as chosen by the user unless the user specified them.

用户探索方向 → 先 brief-first。只有 brief 已可行动、用户明确要找参考、已有模板线索，或模板不会替代关键创意判断时，才做模板来源解析或 inspiration matching。

## Creative Reasoning Spine

Use this spine to shape plans and prompts:

1. **Outcome**: what business/user result the video must create.
2. **Audience state**: what the viewer already believes, needs, doubts, or wants.
3. **Attention**: why the first seconds earn attention.
4. **Clarity**: what must be understood quickly.
5. **Proof**: what product, reference, evidence, transformation, or scene makes the claim believable.
6. **Motion**: what changes across time; avoid static prompts for moving media.
7. **Sound**: whether audio is diegetic, background, voice, silent, or tool-generated by the selected model.
8. **Action**: what the viewer or downstream operator should do next.
9. **Execution risk**: what can fail in model capability, reference alignment, policy, assets, or delivery.

## Mode Contracts

### `brief-to-video-plan`

Deliver:

- goal and audience
- platform, aspect ratio, duration, and format assumptions
- concept and structure
- asset requirements and missing inputs
- recommended model/input mode only when enough information exists
- execution risks and next step
- when the brief is talking-head / UGC monologue / narrated b-roll: apply **Talking-head UGC execution contract** below

Completion evidence is the plan itself. No generation tool or video artifact is required.

### `prompt-only`

Deliver:

- final prompt or prompt pack
- model/input-mode notes when a model is known
- asset-role mapping for each image/video/audio reference
- motion, camera, scene continuity, and sound notes
- parameter suggestions and assumptions
- when the job is talking-head UGC: apply **Talking-head UGC execution contract** (identity, look, captions opt-in, pacing)

Completion evidence is a usable prompt/prompt pack. No generation tool or video artifact is required.

### `reference-analysis`

Deliver:

- reference summary
- structure, hook, pacing, scene logic, product/proof role
- visual, motion, sound, and transition observations
- what can transfer to a new video and what should not be copied
- generation notes only if useful

Completion evidence is the reference breakdown. Do not turn reference analysis into generation unless the user explicitly asks to create a new video.

### `generate-video`, `image-to-video`, `reference-to-video`

Before external generation, confirm the request is actionable enough:

- target result and content type
- input assets and their roles
- platform, aspect ratio, and duration
- selected model/input mode, when model selection is needed
- prompt and core parameters
- cost or external side-effect risk if relevant
- when talking-head UGC: face/identity rules, caption opt-in, and multi-take plan from **Talking-head UGC execution contract**

Useful tool affordances:

- Use `match-media-inspirations` only when a template or inspiration match would materially improve the request.
- For generation, call `generate-video` as the single Agent-facing entry. Pass `modelPreference` and/or `sceneOptionId` when the user or plan has a model preference; the server resolves and freezes the formal model contract.
- Do not invent provider model IDs. Prefer natural-language preferences (Seedance, Kling, Veo, Sora) or a known formal `sceneOptionId`.
- When a target scene option is known, read the matching `video-model-${sceneOptionId}` skill for model-specific prompt and parameter guidance before calling `generate-video`.
- Do not call list/select/init/submit/wait semi-process tools. Those are internal/operator only.

Generation completion evidence requires a real task/result with video references such as `taskId` plus pending delivery or final `videoUrls` / `r2_key`, or a clear blocker from `generate-video`. Do not imply completion when no task/result/blocker exists.

### `variant/revision`

First identify whether the user wants to revise text/planning artifacts or regenerate video.

- If revising a plan/prompt, deliver the revised plan/prompt. No video artifact is required.
- If regenerating video, follow the generation evidence rules.
- If revising a failed generation, use observed failure evidence before changing prompt, assets, model, or parameters.

### `video-diagnosis`

Deliver:

- what is weak or risky
- why it matters for the viewer, model, or delivery
- concrete fix
- whether the fix requires a new prompt, new assets, different model, or regeneration

Completion evidence is the diagnosis and next-step recommendation. No video artifact is required unless the user asks to regenerate.

## Talking-head UGC execution contract

Use when the user wants vertical talking-head, UGC monologue, or narrated product/b-roll review — not for pure multi-shot story ads unless identity/pacing rules still help.

This is an **Agent planning and prompt contract**. OmniMux does **not** provide a single composed “one-tool multi-take stitch” SaaS workflow. Delivery still goes through user-level plan/prompt and optional `POST /v1/video/generations` (see `OMNIMUX.md`). Multi-take may mean multiple generation tasks or one long prompt within model limits — state which path you use.

### Three shapes

| Intent | Minimum inputs | Deliverable shape | Face requirement |
| --- | --- | --- | --- |
| Short talking-head | Full spoken line + optional person description or image | One short take plan (~5–15s by script density) | Optional (default persona text OK) |
| Full monologue | **Complete** monologue text + real face reference | Multi-take plan; **never silently trim** monologue meaning | **Required** (face photo / still URL / prior sheet or locked face ref) |
| Narrated b-roll | Script + b-roll or product footage ref URL + real face | Person presence/voice plan + lower/overlaid footage plan | **Required** |

Script density and take length: prefer sibling pack `video-script-creation` → `docs/talking-head-pacing.md` (EN ~2.0–2.5 words/s; Chinese ~3.5–5 chars/s heuristics). Quick EN guide: 1–11 words ≈5s; 12–22 ≈10s; 23–33 ≈15s; 34+ split takes.

### Identity model

| Input | Meaning |
| --- | --- |
| `person` | Text-only description of who is speaking |
| `image` | Face/photo still (URL or user-provided image) locks appearance |
| `character_reuse` | Reuse the same face ref / portrait / sheet across takes in **this task** |

Rules:

- Pass at most one primary identity strategy per take set unless the user wants a cast change.
- After a face is established, **prefer reuse** on the next take; if unsure same person vs new, **ask once**.
- Full monologue and narrated b-roll need a **real face ref**, not text-only `person`.
- Do not invent third-party host character IDs as OmniMux runtime objects.

### Captions (opt-in)

- Default **off**. Do not burn in captions or caption styles unless the user asks.
- If captions are wanted, ask which style when it changes the look: optional vocabulary `hormozi` | `tiktok` | `minimal` (describe visually if the model has no enum).

### Look presets

| Look | Prompt intent |
| --- | --- |
| `natural` | Soft natural daylight, candid UGC framing |
| `commercial` | Clean, brand-ready, controlled light |
| `raw_iphone` | Unpolished phone-shot feel via lighting/framing — not “holding a phone” as a prop |

Prefer natural skin, believable imperfection, non-plastic faces. Realism is a **prompt/planning** choice on OmniMux, not a hidden server flag.

### Anti-patterns

- Do not write “selfie”, “holding a phone”, or “phone camera” into pose/subject language unless the product **is** a phone demo.
- Prefer “talking to camera”, “eye contact with lens”, “handheld feel via framing and light”.
- Do not auto-enable captions.
- Do not trim a full monologue without explicit user OK.
- Do not route the user into a third-party exclusive UGC SaaS as the default OmniMux path.

## Video Prompt Quality Guards

Use these guards for prompt and generation modes:

- **Frame change**: first and final states must have visible, interpolatable change. Movement, posture, object state, composition, or camera position should change enough for video.
- **Continuity**: key subjects that must remain visible should be declared as present from the start when needed to avoid pop-in/pop-out.
- **Motion physics**: camera movement must be physically plausible for duration and scene scale. Split impossible moves into multiple shots.
- **Reference role clarity**: each image, video, or audio reference needs a role: main subject, style, motion reference, environment, product proof, voice/audio, or excluded context.
- **Sound source**: distinguish visible in-scene audio, background music, voice, model-generated audio, external licensed audio, or silence.
- **Model fit**: prompt and parameters must match the selected model/input mode rather than a generic all-model prompt.
- **Talking-head identity**: lock the same face ref across multi-take UGC when continuity matters; apply look presets and anti-selfie rules from the contract above.

## Verification

### General completion evidence

Use for all modes:

- requested deliverable is present
- material assumptions are stated when used
- missing information that affects success is asked once or reported as blocker
- tool failure, policy block, provider failure, or retry exhaustion is surfaced with observed evidence and next step

For `brief-to-video-plan`, `prompt-only`, `reference-analysis`, and `video-diagnosis`, no video artifact is required. A complete plan, prompt pack, analysis, diagnosis, or clear blocker is enough.

### Generation extension evidence

Use only when the chosen mode requires generation:

- `generate-video` was called only after the request became actionable enough
- model preference or scene option came from current user/plan evidence, not invented IDs
- `generate-video` result is present, or blocker is present
- successful handoff includes real task evidence such as `taskId` with pending delivery, or final `videoUrls` / `videoUrl` / `r2_key`
- queued/running/processing states are reported as in progress, not as completed video delivery

If no generation tool was called because the user asked only for a plan, prompt, reference analysis, or diagnosis, explicitly finish with that deliverable and do not apologize for missing video output.

## Never Do

- Do not turn skill attachment into a fixed tool sequence, workflow route, `toolChoice`, or completion gate.
- Do not call or recommend list/select/init/submit/wait semi-process tools for ordinary generation; use `generate-video`.
- Do not demand a video artifact for `prompt-only`, `reference-analysis`, `brief-to-video-plan`, or `video-diagnosis`.
- Do not silently use schema defaults when they change platform fit, cost, user expectation, or success criteria.
- Do not treat video editing, pure storyboard tables, image creation, slideshow creation, or template matching as video generation unless the user explicitly asks to land a new video.
- Do not auto-enable burn-in captions on talking-head UGC.
- Do not silently trim monologue meaning to fit a single take.
- Do not claim a composed multi-take SaaS pipeline that OmniMux does not expose; plan takes explicitly.
- Do not force exclusive third-party UGC hosts as the default generation path.
