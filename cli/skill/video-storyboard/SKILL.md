---
name: video-storyboard
description: >-
  视频分镜表创作导演：把视频创意、脚本或参考内容拆成可执行的镜头表 / shot list，
  包含镜头编号、时长、叙事目的、画面描述、镜头/动作、旁白/对白、声音提示、转场和生成提示词备注。
  当用户要求视频分镜表、镜头表、脚本拆镜、reference breakdown、AI 视频/生图提示词分镜、
  或诊断分镜节奏与镜头连续性时激活。
  不应激活：直接生成视频成片、剪辑已有视频、画布节点编排、纯文案脚本、分镜图片拼图，
  或只需要选择视频模型/启动视频生成 workflow。
metadata:
  version: "2.0.0"
  author: omnimux
  tags: [视频, 分镜表, 镜头表, shot-list]
  category: content-generation
  qualityChecks:
    - shot_table_complete
    - timing_consistent
    - generation_boundary_clear
  publicationStatus: public
  codeExecutionCore: false
  compatibility: OmniMux CLI flat skill under cli/skill/video-storyboard/. Pair with sibling skill `omnimux`.
---

# Video Storyboard Table Skill

## OmniMux shared

开始前阅读 sibling skill **`omnimux`**（CLI 安装、密钥、`tokens exec`）。不要在对话中暴露 `sk-`。可选生成见 [`OMNIMUX.md`](OMNIMUX.md)。



## Positioning

This skill is a **视频分镜表创作导演**. It turns a video idea, script, rough brief, or reference material into a structured storyboard table that a creator, editor, or generation workflow can understand.

It is not a video generation button, a canvas node planner, a video editor, or an image-grid composer. Use it to decide and document what each shot should communicate before any optional generation step.

Related references:

- Style presets: [`references/script-styles.json`](references/script-styles.json)
- Output contract: [`references/output-contract.md`](references/output-contract.md)
- Examples: [`references/examples.md`](references/examples.md)
- Publication governance: [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md)

## Publication Governance

This skill is a public business-methodology skill, not a code-execution skill. The publication readiness record lives in [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md).

Do not turn this storyboard method into a fixed workflow, required-next tool, sticky skill state, or generation completion gate. Generated images, videos, canvas persistence, or exports require separate explicit user intent and real tool evidence.

## Task Modes

| Mode | Use when | Primary deliverable | Requires image/video artifact |
| --- | --- | --- | --- |
| `brief-to-storyboard` | User gives a topic, product, campaign idea, or rough creative direction. | A complete storyboard table from brief to shots. | No |
| `script-to-storyboard` | User already has narration, dialogue, voiceover, or a script draft. | Shot table aligned to the script beats. | No |
| `reference-breakdown` | User asks to study a reference video, shot sequence, or content structure. | Reference mechanics breakdown plus an original shot table direction. | No |
| `shot-prompt-pack` | User wants the storyboard to be ready for AI image/video prompting. | Storyboard table plus per-shot generation prompt notes. | No |
| `variant/revision` | User wants another version, shorter duration, different audience, or revised tone. | Updated storyboard table with explicit changes. | No |
| `storyboard-diagnosis` | User asks whether a storyboard/script will work visually or structurally. | Diagnosis with concrete shot-level fixes. | No |

If the user explicitly asks to generate frames, images, or a final video after the table is ready, treat that as a separate generation request and require real tool/artifact evidence for that generation step.

## Intake Rules

Ask one high-value question before drafting when missing information would materially change duration, platform, aspect ratio, shot density, output format, cost, or success criteria.

Proceed with stated assumptions only when the user clearly says to "先出一版", "直接给我一版", "你定", or an equivalent direct-draft instruction. State the assumptions at the top of the response and keep them easy to revise.

Prefer questions that unlock multiple downstream decisions:

| Missing decision | Good question |
| --- | --- |
| Platform and aspect ratio | "这条分镜表主要用于 TikTok/Reels/Shorts 竖屏，还是横屏宣传片？" |
| Duration and shot density | "目标时长大概是 15 秒、30 秒还是 60 秒？" |
| Business goal | "这条视频更偏转化、品牌种草、教程，还是知识讲解？" |
| Source material | "你希望我按现有脚本拆镜，还是从 brief 重新设计镜头表？" |

Do not invent hidden defaults for platform, duration, aspect ratio, product proof, or success criteria when those choices would change the storyboard.

## Storyboard Table Contract

A complete storyboard table should include these columns unless the user asks for a lighter format:

| Column | Meaning |
| --- | --- |
| `shot` | Shot number or scene id. |
| `duration` | Approximate seconds for the shot. |
| `purpose` | Narrative or conversion job of the shot. |
| `visual` | Concrete visual description: subject, setting, action, state change, and visible proof. |
| `camera_action` | Shot size, camera movement, blocking, or transition-relevant action. |
| `voice_or_dialogue` | Voiceover, on-screen dialogue, or silence. |
| `sound` | BGM, sound effect, ambience, or audio boundary note. |
| `transition` | Cut, match cut, wipe, hold, speed ramp, or continuity note. |
| `prompt_note` | Optional generation-ready prompt note for image/video tools. |

For `shot-prompt-pack`, make `prompt_note` specific enough for a generation model, but do not claim generation has happened unless a generation tool actually returns an artifact.

## Creative Method

Use the five-role lens to shape the table:

| Lens | Question |
| --- | --- |
| Strategy director | Who is watching and what action should they take? |
| Writer | What is the hook, turn, proof, and payoff? |
| Director | What must be visible in each shot? |
| Cinematographer | What camera distance, movement, and continuity make the shot executable? |
| Sound designer | What voice, dialogue, silence, music, or sound effect supports the shot? |

For commerce or UGC tasks, keep product visibility and proof explicit. For educational or tutorial tasks, keep sequence and comprehension explicit. For narrative tasks, keep emotional transition and continuity explicit.

## Boundaries

Do:

- Produce a shot table, storyboard diagnosis, or prompt-ready shot plan.
- Split existing scripts into visual beats.
- Analyze reference mechanics and rebuild an original shot plan.
- Mark open questions and assumptions clearly.

Do not:

- Start or imply a video generation workflow just because this skill is attached.
- Require `video-creation`, `canvas-add-storyboard-results`, `ai_video_storyboard_analyze`, `/api/video-storyboard`, or any other specific tool.
- Treat a storyboard table as a persisted canvas result unless a tool actually persisted it.
- Present a frame grid, video file, or rendered storyboard image as complete unless that artifact exists.
- Convert "make a storyboard table" into "generate a finished video".

## Common Failure Modes

| Failure | Why it is wrong | Fix |
| --- | --- | --- |
| Only writing a script | The user asked for shots, not only copy. | Add visual, camera/action, sound, and transition columns. |
| Generic visuals | "Person talks to camera" is not production-ready. | Specify subject, setting, action, state change, and visible proof. |
| Hidden generation assumption | A table does not prove images or video exist. | Separate planning evidence from generation artifact evidence. |
| Over-dense shots | Too many visual actions in one shot make execution unclear. | Split or simplify shots. |
| Missing audio boundary | Voiceover, dialogue, silence, and BGM compete. | Mark the intended audio role per shot. |

## Verification

General completion evidence:

- [ ] Task mode is clear or assumptions are stated.
- [ ] The answer contains a structured storyboard table or a diagnosis tied to shot-level fixes.
- [ ] Each shot has duration, purpose, concrete visual action, audio/voice direction, and transition or continuity notes.
- [ ] Total duration and shot density are internally consistent with the stated goal.
- [ ] Any open questions are listed without blocking a requested first draft.

Generation extension evidence:

- [ ] If the user asked for generated frames/images/video, the response cites the actual returned artifact, URL, file, task id, or clear blocker from the generation tool.
- [ ] If no generation tool was called, the response states that the deliverable is a storyboard table / prompt pack, not a generated artifact.
- [ ] A finished video, frame grid, or canvas result is never claimed without concrete artifact or persistence evidence.
