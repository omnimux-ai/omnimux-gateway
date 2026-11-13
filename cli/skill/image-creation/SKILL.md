---
name: image-creation
description: >-
  图片创作：视觉资产创作导演。启用：封面、海报、插画、社交视觉、产品图、
  AI art、prompt 优化、参考图改版、视觉诊断。不启用：infographics、
  slideshow、video、template editing。
metadata:
  version: "4.1.2"
  author: omnimux
  publicationStatus: public
  codeExecutionCore: false
  tags: [image, visual-asset, prompt-engineering, creative-direction]
  category: content-generation
  compatibility: OmniMux CLI flat skill under cli/skill/image-creation/. Pair with sibling skill `omnimux` for models/tokens/API execution.
---

# Image Creation Skill

视觉资产创作导演：把用户的视觉意图转成可判断、可生成、可改版或可诊断的视觉交付物。先判断任务模式，再决定是否调用生成工具；不要把本 skill 当成固定图片生成流程。

## Publication Governance

This skill is a public business-methodology skill, not a code-execution skill.

- Publication readiness record: [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md).
- Prompt writing, visual brief creation, reference-guided planning, and diagnosis complete through text contracts.
- Image generation happens only when the user explicitly asks for an image artifact and a visible generation tool returns evidence.
- No bundled script or harvested prompt corpus is part of the public runtime contract.
- Do not invent gallery matches, source URLs, reference images, or prompt examples. Only use evidence from the current turn, visible tools, or explicitly loaded references.
- Do not create a fixed tool sequence, hidden route, sticky skill state, or product-specific completion gate because this skill is selected.
- Host tools are not part of this package's contract: only call a generation capability that is actually visible in the current environment; if none exists, deliver the prompt/brief and state the blocker. Runtime binding: [`OMNIMUX.md`](OMNIMUX.md).

## Core Contract

用户要的是视觉资产能力，不一定要立刻生成图片。先确认交付物：

- 只要 prompt、视觉 brief 或模型参数时，交付文本合同。
- 明确要出图、先出一版、生成多版或继续生成时，才调用生成工具。
- 参考图、灵感模板和现有素材只作为当前轮证据；没有真实证据时不要伪造匹配。
- 信息图、多页图文、视频生成、已有模板编辑不属于本 skill，除非用户明确只需要其中的单个视觉元素创意。

## Task Modes

| Mode | Use When | Deliverable | Generate? |
| --- | --- | --- | --- |
| `prompt-only` | 用户要提示词、视觉 brief、风格方向、参数建议 | 英文 prompt、中文解释、aspectRatio、文字/参考图约束 | No |
| `generate-artifact` | 用户明确要生成、出图、先做一版、生成多版 | 可见生成调用（如 `omnimux tokens exec` + `POST /v1/images/generations`）的结果与生成说明 | Yes |
| `reference-guided` | 用户提供参考图、灵感模板、商品图、风格参考 | 保留事实的参考图创作方案；需要出图时传 `referenceImageUrls` | Maybe |
| `poster/social-visual` | 用户要封面、海报、社交视觉、产品图、活动视觉 | 视觉结构、文案层级、构图、prompt 或生成结果 | Maybe |
| `variant/revision` | 用户要改一版、换风格、扩展多个方向 | 变化点、保持项、改版 prompt 或新生成结果 | Maybe |
| `visual-diagnosis` | 用户要分析图片、判断好坏、找改进点 | 问题诊断、修改优先级、可执行改进 brief | No |

## Brief-First Gate

本节是 image creation 的 brief-first gate；它先判断交付物和风险，再决定是否进入 prompt 构建、风格选择、gallery 浏览或生成。

在 prompt 构建、风格选择、灵感参考或生成之前，判断请求是否 Actionable Enough。

Material Uncertainty 检查维度：

- 目标结果或用途、受众/观者状态、主体/产品事实/核心信息
- 渠道/格式/画幅期望、已提供素材或参考图
- 约束、风险、成本、外部副作用、成功信号

当缺失信息会改变用途、画幅、成本、交付物或成功标准时，先问一个高价值问题，或给 2-3 条路径菜单。不要默默选择默认类型、风格、模板或生成工具。

只有当用户明确说“直接生成”“先出一版”“你先做一个方向”这类可以承受付费/外部生成副作用的请求时，才带声明假设推进。声明格式：

```text
我先按 [用途/画幅/风格] 推进；如果你要换成 [关键分歧]，下一轮我再改。
```

## Visual Type Classification

根据上下文判断视觉类型；这只是 prompt/构图参考，不是强制路由。

| Type | Intent Signals | Typical aspectRatio |
| --- | --- | --- |
| Cover | 封面、文章配图、banner、thumbnail | 16:9 |
| Poster | 海报、宣传、活动、促销 | 3:4 |
| Social Card | 小红书配图、朋友圈、社交媒体 | 1:1 or 3:4 |
| Illustration | 插画、插图、概念图、配图 | 1:1 |
| Portrait | 头像、人物照、证件照 | 1:1 |
| Product | 产品图、电商、商品展示 | 1:1 or 3:4 |

类型不明确时不要默认 Illustration。若用户允许先出方向，选择最保守的假设并说清楚。

## Prompt Engineering

### Step 1: 选择模式与维度

1. 选择 Task Mode。
2. 判断视觉类型和 aspectRatio；用户覆盖优先。
3. 加载 5 维默认值：Type / Palette / Rendering / Text / Mood，见 [references/prompt-dimensions.md](references/prompt-dimensions.md)。
4. 输出文本还是调用工具，由 Task Mode 和用户明确意图决定。

### Step 1.5: 灵感与参考证据

仅当当前轮有真实灵感来源时使用：用户消息含 `<selected_inspiration_template>`、已绑定模板、会话上下文含可用灵感摘要数据、用户提供参考图，或工具返回候选且用户确认。

使用时：

- 用户主体、产品事实、人物、文字要求和显式约束是真相源。
- 灵感骨架只用于构图、风格方向、镜头结构、情绪和兼容参考媒体。
- 可用图片参考通过实际 API 的参考图参数传递。
- 仅有 prompt/分析文本时用于增强英文 prompt，不伪造图片 URL。

无灵感证据时跳过，不阻塞创作，不强制模板匹配。

### 电商海报分支

意图信号为电商海报、产品推广、店铺装修、banner、商品展示时，按需读取 [references/ecommerce-poster-prompt-pack.md](references/ecommerce-poster-prompt-pack.md)。严格保留产品事实（外观、颜色、材质、规格）；创意自由度只用于背景、氛围、造型和视觉层级。

### Step 2: 应用风格

- 用户提到风格时，匹配 [references/style-gallery.md](references/style-gallery.md) 预设风格；Style 覆盖 Palette + Rendering + Mood，Type + Text 仍来自交付物。
- 用户未提风格时，用 5 维默认值；用户给出部分线索时只覆盖对应维度。

### 合规分支

主体涉及人物角色（尤其肖像/lookbook）、gore/暴力/暗黑幻想、真实公众人物、品牌名/商标/logo 时，读取 [references/compliance-rewrite-guide.md](references/compliance-rewrite-guide.md)。应用改写公式：欲望词转美学词，描述全身姿态而非局部，服装得体、场景明亮、构图 editorial。

### Step 3: 构建 Prompt

结构：

```text
[Rendering] [visual type composition] of [Subject].
[Palette] color scheme.
[Mood] visual tone.
[Text instructions if applicable].
[User-specific details and constraints].
```

核心要求：

1. Prompt 用英文；如需要中文画面文字，用英文说明并把中文文案放在引号中。
2. 主体具体，不写抽象空词。
3. 相关时包含构图、镜头角度、取景和空间关系。
4. 人物主体必须包含成人声明（`adult, 20+ years old`）、健康比例、得体姿态，不描述局部。
5. 用户提供参考图时，通过实际 API 的参考图参数传递，不在 prompt 里二次描述参考图视觉特征。
6. `prompt-only` 模式也要给可直接复制到图像模型的英文 prompt。

短或模糊描述时，按需读取 [references/prompt-enhancement-guides.md](references/prompt-enhancement-guides.md)，选择 Realistic Blueprint、Anime Creative Expansion 或 Illustration Art Brief。

## Generate When Needed

仅在 `generate-artifact` 模式，或其他模式中用户明确要求出图时，才通过可见工具或 API 生成；宿主不提供生成能力时，交付可直接使用的 prompt 并说明 blocker。

生成调用参数（按实际可见工具/API 的 schema 调整）：

- `prompt`：2-5 句英文视觉 prompt。
- `aspectRatio`：用户指定优先；否则来自已声明的视觉类型假设。
- `includeTextInImage`：仅当 Text 维度不是 `none` 时为 true。
- 参考图参数：用户提供的参考图、商品图或可见工具返回的可用图片 URL。
- `contentCategory`：按实际用途映射为 educational / marketing / personal。

多图请求：

- 每张独立调用生成工具。
- 保持统一 aspectRatio 与核心风格维度。
- 每张必须有独立 artifact 证据；不要用一次成功代表全套成功。

生成失败：

- 普通失败：简化 prompt 后最多重试 1 次。
- safety / content_policy / NSFW：不要简单化重试；按 compliance guide 改写后最多重试 1 次。
- 仍失败时给出 provider/task 错误、已尝试 prompt、下一步建议。

## Output Contracts

### `prompt-only`

- 任务判断：用途、受众、视觉类型、aspectRatio 假设。
- 英文 prompt。
- 可选参数：negativePrompt、includeTextInImage、参考图参数使用建议。
- 不调用生成工具。

### `generate-artifact`

- 任务判断与声明假设。
- 英文 prompt 与关键参数。
- 真实生成 artifact：`imageUrl`、task id、provider status 或明确失败证据。
- 多图时逐张列出 artifact。

### `reference-guided`

- 参考证据来源。
- 必须保持的事实。
- 可变创意空间。
- prompt 或生成结果，取决于用户是否要求出图。

### `poster/social-visual`

- 视觉目标、版式结构、文字层级、主体关系。
- prompt 或生成结果。
- 说明哪些文字适合进图，哪些建议后期排版。

### `variant/revision`

- 保持项。
- 变化项。
- 新 prompt 或新生成 artifact。
- 若用户只要建议，不生成。

### `visual-diagnosis`

- 诊断维度：主体清晰度、构图、色彩、文字、品牌/商品表达、平台适配。
- 优先级排序的修改建议。
- 可执行改版 prompt 或 brief。
- 不要求真实图片 artifact。

## Quality Standards

交付前检查：

1. 模式正确：输出类型与用户请求一致，没有把分析/诊断误升级为生成。
2. 关键不确定性已处理：会改变用途、画幅、成本或成功标准的问题已澄清或声明假设。
3. prompt 具体：主体、环境、风格、构图、文字要求明确。
4. 风格一致：Rendering + Palette + Mood 不互相冲突。
5. 参考证据真实：参考图或灵感骨架来自当前轮可验证来源。
6. 无噪音词：不用 "hyper-realistic" / "8K ultra HD" / "masterpiece" / "best quality"。
7. 合规分支落实：人物、名人、暴力、品牌 logo 等风险已按合规参考改写。

## Verification

通用 done 前确认：

- [ ] Task Mode 已声明或可从输出看出。
- [ ] 交付物匹配用户请求：prompt / brief / 诊断 / 改版 / 生成结果之一。
- [ ] 如有假设，已明确说明。
- [ ] 如使用参考证据，来源真实且未伪造 URL。
- [ ] 如输出 prompt，主 prompt 为英文。
- [ ] 如涉及合规风险，已应用合规改写。

生成模式额外确认：

- [ ] 生成调用返回 `imageUrl`、task id、provider status 或明确失败证据。
- [ ] 多图请求每张都有独立 artifact 或失败证据。
- [ ] 失败时已说明 provider/task 错误、已尝试处理和下一步。
