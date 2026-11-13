---
name: social-card
description: >-
  社交图文创作导演：小红书/抖音/WeChat 图文套组、3:4 封面、微信公众号封面、
  carousel 卡片、参考拆解、文案包、视觉方向、渲染交付与诊断。
  当用户要求社交图文、小红书卡片、carousel、公众号封面、Swiss Style 图文或杂志风卡片时激活。
  不应激活：PPT、视频、信息图、普通图片生成、纯文本脚本、已有模板编辑，或只做链接保存/素材整理。
metadata:
  version: "4.0.2"
  author: omnimux
  publicationStatus: public
  codeExecutionCore: false
  tags: [social-card, xiaohongshu, wechat, editorial, swiss]
  category: content-generation
  compatibility: OmniMux CLI flat skill under cli/skill/social-card/. Pair with sibling skill `omnimux` for models/tokens/API execution.
---

# Social Card Creation Director Skill

社交图文创作导演负责把选题、素材、链接或参考卡片转成可发布的社交图文方案、文案包、视觉方向、渲染卡片或可复用模板。先判断任务模式，再决定是否调用渲染或持久化工具。

选择 `social-card` 只表示本条消息附加社交图文方法论，不等于必须渲染 PNG，不等于必须调用渲染或持久化能力。

## 公开治理边界

这个 skill 是公开业务方法论 skill，不是代码执行 skill。

- 完整发布准入记录见 [`PUBLICATION-GOVERNANCE.md`](PUBLICATION-GOVERNANCE.md)。
- 非渲染模式可以只输出策划、文案、参考拆解、视觉方向或诊断，不需要运行任何代码。
- `assets/magazine-bg-webgl.js` 是模板内联背景资产，只能作为渲染 HTML 的素材，不是 Agent runtime 的隐藏执行入口。
- `validate-social-deck.mjs` 是版面质量校验器的参考实现/本地工具支撑；只有用户或可见工具链明确进入渲染验收时才使用，不得因为 skill 被激活就自动执行。
- 渲染、持久化、校验等能力必须通过可见工具或命令合同调用；不要把本 skill 写成固定工具序列、隐藏阶段机或持久化默认流程。
- 完成证据按任务模式判断：文案/方案/诊断不要求 PNG；渲染模式才要求真实 PNG；模板持久化模式才要求 template ID。
- 宿主工具不是本包契约：只有当前环境真实可见的渲染、持久化、搜索或生成能力才调用；缺失时交付对应非渲染模式并说明 blocker。Runtime binding: [`OMNIMUX.md`](OMNIMUX.md).

## 产出范围

**适用**：小红书/Rednote 3:4 卡片套组（封面 + 内容页）、微信公众号 21:9+1:1 封面配对、产品截图帖、教程轮播、户外/生活笔记。

**不适用**：PPT / 视频 / 信息图 / 普通图片生成 / 纯图片编辑 / 已有模板编辑 / 纯摄影展示帖（无排版需求）/ 只保存链接或整理素材。

**能力圈边界**：旅行/职场/推荐 强端到端；游戏/影视/美食食谱/彩妆教程/健身/家居/穿搭精选 文本和结构强但图需用户提供；美食大片/日常 OOTD/梦核/Y2K 超出能力圈，应在 intake 时诚实说明而非静默交付不达预期的结果。详见 `references/category-cookbook.md`。

## 任务模式

| Mode | 用户真实需求 | 标准交付物 | 需要渲染 PNG | 需要持久化模板 |
|------|--------------|------------|--------------|----------------|
| `brief-to-card-plan` | 从选题、产品、链接或粗 brief 做社交图文策划 | 平台、页数、封面 hook、每页叙事目的、素材需求、视觉方向 | No | No |
| `copy-pack` | 只要标题、正文、封面文案、CTA、标签 | 可直接粘贴的文案包和页面文案，不交付图片 | No | No |
| `reference-breakdown` | 拆解参考帖、URL、截图或竞品卡片 | 结构拆解、视觉语言、可复用版式、注意事项 | No | No |
| `visual-direction` | 确定风格、版式、配色、素材策略 | 风格板、布局建议、图片来源策略、生成/搜索提示词 | No | No |
| `render-card-set` | 明确要图片成品、PNG、封面或卡片套组 | 渲染后的 PNG URL、页数、尺寸和图片来源说明 | Yes | No |
| `template-persist` | 明确要可复用模板、保存到项目或进入模板库 | PNG URL + 持久化模板 ID + 页面结构说明 | Yes | Yes |
| `variant/revision` | 对已有方案、文案或卡片做改版 | 改版差异说明和对应交付物；是否渲染/持久化取决于原始请求 | Conditional | Conditional |
| `social-card-diagnosis` | 诊断已有图文、草稿或渲染结果 | 问题清单、优先级、修改建议和验收标准 | No | No |

模式判断规则：
- 用户说"写一套文案"、"给我方案"、"拆一下参考"、"诊断一下"时，默认不渲染、不持久化。
- 用户说"生成图片"、"做出卡片"、"给我 PNG"、"出封面图"时，进入 `render-card-set`。
- 用户说"保存模板"、"可复用模板"、"进项目模板库"、"后面还能改"时，进入 `template-persist`。
- 如果用户只说"做一组小红书图文"且交付形态会影响成本、工具调用或成功标准，先问一个高价值问题：要先出策划/文案，还是直接渲染图片？
- 只有用户明确"直接做一版/先出一版/不用问"时，才带声明假设推进；持久化仍需用户明确要模板或项目交付。

## Brief-First Intake (Medium Freedom)

收集会改变产出的缺失信息：

1. **目标平台和画幅**：小红书 3:4、抖音、微信 21:9+1:1
2. **内容来源**：用户文本 / URL / 截图 / 字幕 / 照片
3. **小红书品类**（若用户指定 11 类之一）：路由到 `references/category-cookbook.md` 确认能力圈
4. **已有图片/截图及位置**：新闻/教程/数据/评测类内容应主动提示提供截图作为证据层
5. **风格偏好**：Swiss Style / Editorial / 科技 / 户外
6. **硬约束**：标题文字、无图 1:1 封面、必须含硬件照片、截图可读性等
7. **交付模式**：只要方案/文案/诊断，还是要渲染 PNG，是否要持久化模板

**无图时的三选一（one-shot，不重复追问）**：

```text
这篇我需要 1-2 张图。三种走法：
A. 你自己有照片/截图，传给我（推荐——最不"AI 感"）
B. 我去 Pexels/Unsplash/Flickr 帮你找
C. 用 AI 生成
```

用户说"都行"或"不要图片"视为已回答，不再追问。

澄清问题只问会改变用途、画幅、成本、工具调用、交付物或成功标准的一项。不要为了填表连续追问。

## 风格模式 (Medium Freedom)

每套卡片选一个模式，不混用：

- **Editorial Magazine × E-ink**：Songti/Serif 标题 + 安静无衬线正文，纸张 + 墨色调，纸张纹理/墨洗/WebGL 氛围层，杂志特稿感——适合需要慢读、手工排版感的内容
- **Swiss International**：Inter/Helvetica，极轻大字重标题 + 等宽标签，严格左对齐网格，单一高饱和强调色，卡片填充矩阵/KPI 塔/条形图——适合工程感、量化、果断的内容

选择依据是想要的感觉（feature story vs release note），不是内容类型。

**主题选择**：
- Editorial：6 个杂志调色板之一（Ink Classic / Indigo Porcelain / Forest Ink / Kraft Paper / Dune / Midnight Ink）
- Swiss：4 个强调色之一（IKB Blue / Lemon Yellow / Lemon Green / Safety Orange）

详见 `references/theme-presets.md`。

## 页面规划 (High Freedom)

**小红书**：首页强 hook，2-N 页每页一个观点，5-9 页为主，内容不足时压缩合并而非留空。

**微信**：必须产出 21:9 主封面 + 1:1 方封面配对，同一 HTML 中组合预览。21:9 保留完整标题和副标题；1:1 用从长标题提炼的简化短标题（详见 `references/title-shortener.md`）。

用 `references/layout-recipes.md` 选页面结构，避免每页都是"标题+卡片"重复。3:4 图检查 `references/portrait-fill.md` 避免纵向空间填充不足。

## 工具使用边界 (Low Freedom)

### 非渲染模式

`brief-to-card-plan`、`copy-pack`、`reference-breakdown`、`visual-direction`、`social-card-diagnosis` 可以直接用结构化文本完成。不要为了满足 skill 选择而调用渲染或持久化能力。

### 渲染模式

`render-card-set` 使用 seed 模板作为设计脚手架：
- Editorial → `assets/template-editorial-card.html`（直接读取本包文件）
- Swiss → `assets/template-swiss-card.html`（直接读取本包文件）

Seed 已包含字体加载、主题 token、三种海报尺寸、背景层和所有 class 定义。在 `<!-- POSTERS_HERE -->` 后替换为每页一个 `<section class="poster">` 块。不要用错误的模板 class 系统（Editorial 用 serif+ledger/marginalia，Swiss 用 Inter+card-fills/matrix）。

构建与渲染：
1. 构建完整单文件 HTML（inline CSS + 每页一个 poster section）
2. 用可见的 HTML→PNG 渲染能力输出：`width: 1080, height: 1440, deviceScaleFactor: 1`；宿主没有渲染能力时停在 HTML 交付并说明 blocker
3. 验证尺寸和渲染结果
4. `done` 引用渲染的 PNG URL、页数、尺寸和图片来源说明

### 持久化模式

`template-persist` 只有在用户明确要可复用模板、保存到项目或进入模板库时使用。

持久化步骤：
1. 先完成 `render-card-set`
2. 用可见的持久化能力写入：每页一个 `pages[]` + `resolvedImages[]`，`aspectRatio: "3:4"`，`replicationMode: "user_skipped"`；宿主没有持久化能力时只交付 PNG + 页面数据并说明 blocker
3. `done` 引用渲染 PNG URL 和模板 ID

"小红书图文"/"社交图文卡片"不自动等于持久化模板。只有用户要求图片成品时才要求 PNG；只有用户要求模板/保存时才要求模板 ID。

### 图文叠加规则

文字在照片上时遵循 `references/image-overlay.md`：
- **选择优先，仅必要时加 tint**：先不加遮罩，缩略图检查失败再加局部图片色调 tint
- **Subject mapping 必做**：记录主体位置为 HTML 注释，文字只放在安全区
- **object-position 内联**：每张照片根据主体位置写 inline style，不用默认 center 50%
- **Thumbnail test**：缩小到 360px 宽确认标题仍可读

## 图片来源 (Medium Freedom)

**用户提供截图**：保留内容、程序化 framing（目标比例画布+安全边距+干净背景）、不拉伸。

**用户无图且需要图**（三选一选了 B 或 C）：
- **B（网络搜索）**：用可见的图片搜索能力（Unsplash/Pexels/Flickr/Wallhaven 等来源），中文关键词加 `locale: "zh"`。返回 URL 直接用。取图后向用户披露来源并询问是否标注。详见 `references/production-workflow.md`。
- **C（AI 生成）**：用可见的图片生成能力（如 `omnimux tokens exec` + `POST /v1/images/generations`），仅 1-2 页需要时生成，匹配页面视觉角色而非通用装饰。

**URL 内容提取**：社媒 URL（X/小红书/TikTok/抖音/Instagram）与普通网页都用当前环境可见的 URL 抓取能力；没有可用能力时请用户提供正文或截图。提取阶段最多 3 次成功调用，超过则停下问用户。

## 完成证据 (Low Freedom)

### 通用完成证据

所有模式都要能说明：
- 已识别任务模式和交付物
- 已处理影响结果的关键约束，或明确说明采用的假设
- 输出与用户请求一致：方案、文案、拆解、视觉方向、图片成品、模板或诊断
- 如果缺少必要素材、权限、工具或外部页面访问失败，给出清楚 blocker 和下一步

### 非渲染模式证据

`brief-to-card-plan`、`copy-pack`、`reference-breakdown`、`visual-direction`、`social-card-diagnosis` 完成时不要求 rendered PNG，也不要求 template ID。有效完成证据分别是：
- 策划模式：平台、页数、封面 hook、每页目的、素材清单和视觉方向
- 文案模式：封面标题、每页正文、CTA、标签和可选发布说明
- 参考拆解：结构、节奏、版式、视觉语言和可复用规则
- 视觉方向：风格、字体/配色方向、布局策略、图片来源或提示词
- 诊断模式：问题清单、优先级、修改建议和验收标准

### 渲染模式额外证据

`render-card-set` 完成前确认：
- [ ] **渲染成功**：可见渲染能力返回 PNG URL
- [ ] **3:4 卡片密度**：内容覆盖 ≥75% 画布高度，无 >15% 画布高度的纯空白带（除非 hero image 自带呼吸或单句宣言）
- [ ] **图文叠加检查**（若适用）：subject mapping 已做，thumbnail test 通过
- [ ] **图片来源披露**（若网络取图）：已向用户说明来源和标注决定

### 持久化模式额外证据

`template-persist` 完成前确认：
- [ ] **渲染成功**：可见渲染能力返回 PNG URL
- [ ] **持久化成功**：可见持久化能力返回 template ID
- [ ] **页面数据完整**：每页都有 `pages[]`、`resolvedImages[]`、`aspectRatio: "3:4"` 和必要文案

如果只选中了 `social-card`，但用户请求属于策划、文案、拆解、视觉方向或诊断，不要把缺少 PNG 或 template ID 当成未完成。

## Reference 文件索引

按需读取，不预加载：

| 文件 | 何时读取 |
|------|----------|
| `references/platform-specs.md` | 需要精确比例、输出尺寸 |
| `references/style-system.md` | 需要 Editorial/Swiss 视觉规则 |
| `references/theme-presets.md` | 选择调色板或强调色 |
| `references/layout-recipes.md` | 选择页面结构 |
| `references/components.md` | 字体栈、字号、间距 token |
| `references/background-systems.md` | 构建 WebGL/ink/paper 背景 |
| `references/portrait-fill.md` | 3:4 纵向填充适配 |
| `references/content-planning.md` | 封面 hook、页面拆分、文案压缩 |
| `references/production-workflow.md` | HTML/CSS 渲染和图像处理 |
| `references/image-overlay.md` | 文字在照片上的处理 |
| `references/screenshot-treatment.md` | 用户截图的处理 |
| `references/map-component.md` | 空间关系（旅行路线、门店位置） |
| `references/title-shortener.md` | 微信 21:9+1:1 封面短标题 |
| `references/category-cookbook.md` | 小红书品类路由和能力圈 |
| `references/qa-checklist.md` | 交付前检查 |
