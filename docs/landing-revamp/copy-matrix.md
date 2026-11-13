# OmniMux 官网落地页重构 — 文案与 i18n 矩阵规范

> **历史材料（2026-10-08 标识）：** 本文仅保留当时的方案、证据与审查结论，不是现行产品要求或当前上线证明。旧的“无订阅”、AI UGC 团队、策划到发布复盘全链路、商品/SKU 起步、旧页面结构与路径、用户手选模型/分组规则不得据此恢复；技术请求与素材契约不因品牌收敛而自动改变。当前依据见 [品牌规范](../brand/README.md) 与 [落地页文案与实现](../brand/landing-page-copy.md)。

> 产品经理：许清楚（Xu） · 交付工件 · 归档 worktree: `.worktrees/landing-revamp/docs/landing-revamp/copy-matrix.md`

---

## 一、核心产品决策记录

### 1. R3 平台矩阵范围决策
- **决策结论**：**保留全部 18 个海外平台，分 3 行展示；第一行严格锁定 6 个官方支持核心渠道**。
- **行布局结构**：
  - **行 1（核心白名单）**：`X (Twitter)`、`LinkedIn`、`YouTube`、`Instagram`、`TikTok`、`Threads`
  - **行 2（主流海外社群）**：`Facebook`、`Discord`、`Telegram`、`Reddit`、`Bluesky`、`Pinterest`
  - **行 3（工作流与去中心化）**：`Slack`、`Mastodon`、`Dribbble`、`VK`、`Lemmy`、`Warpcast`
- **红线核验**：18 个平台全部为海外知名平台，**严禁且零包含中国大陆境内平台**（无微信/抖音/小红书/B站等），完全符合定位约束。
- **副标题定调**：「一站式接入全球主流社交媒体平台，将账号管理、内容创作与数据洞察无缝集成，让全链路运营效率倍增」，兼顾矩阵视觉冲击力与产品事实表述。

### 2. R4 Hero 标题与打字机定位决策
- **决策结论**：废弃 Demo 原有 "OmniMux Desktop" 单纯桌面端定位，**严格对齐官方品牌定位中英基线**。
- **定位口径**：
  - **ZH H1**：`TikTok营销内容自动化`
  - **ZH Sub**：`围绕每个商品，持续生产、审核、发布并迭代 TikTok 营销内容，按调用付费、无需订阅。`
  - **EN H1**：`TikTok marketing content automation`
  - **EN Sub**：`Produce, review, publish, and iterate TikTok marketing content for every product. Pay per call, no subscription.`
- **打字机 5 条短语（Typewriter Phrases）**：
  1. `TikTok营销内容自动化` / `TikTok marketing content automation`
  2. `一站式短视频创意与生成编排` / `Short-form video creation and orchestration`
  3. `多平台自动化发布与矩阵调度` / `Multi-platform automated distribution`
  4. `全模态 AI 生产工作流调度引擎` / `Multimodal AI production workflow engine`
  5. `按调用付费 · 无需订阅 · 即刻接入` / `Pay per call · No subscription · Start instantly`

---

## 二、全量文案矩阵清单（6 屏 + Header/Footer）

| 模块 / 屏 | i18n Key | 中文（ZH） | 英文（EN） | 说明 |
|---|---|---|---|---|
| **Header** | `omnimux_landing.nav.docs` | 文档 | Docs | 外部链接 |
| | `omnimux_landing.nav.skills` | 能力 | Capabilities | 锚点至插件广场 |
| | `omnimux_landing.nav.pricing` | 定价 | Pricing | 内部路由 |
| | `omnimux_landing.nav.apiUpdates` | 更新日志 | API Updates | 外部链接 |
| | `omnimux_landing.nav.github` | GitHub | GitHub | 外部开源仓库 |
| | `omnimux_landing.nav.resources` | 生态与资源 | Resources | 下拉菜单 |
| | `omnimux_landing.nav.login` | 登录 | Log in | 内部路由 |
| **Screen 1 Hero** | `omnimux_landing.hero.h1` | TikTok营销内容自动化 | TikTok marketing content automation | 官方主标 |
| | `omnimux_landing.hero.subline` | 输入一个商品链接，产出一个可发布的爆款作品 | Enter a product link, produce a publish-ready viral creative | 官方副标 |
| | `omnimux_landing.hero.brandAccent` | Desktop | Desktop | 品牌尾缀 |
| | `omnimux_landing.hero.downloadMac` | 下载 macOS 版 | Download for macOS | Mac 主按钮 |
| | `omnimux_landing.hero.downloadWin` | 下载 Windows 版 | Download for Windows | Win 主按钮 |
| | `omnimux_landing.hero.macArm` | Mac (Apple Silicon 芯片) | Mac (Apple Silicon) | M 系列选项 |
| | `omnimux_landing.hero.macReq` | M1 / M2 / M3 / M4 · macOS 13+ | M1 / M2 / M3 / M4 · macOS 13+ | 系统要求 |
| | `omnimux_landing.hero.macIntel` | Mac (Intel 芯片) | Mac (Intel Chip) | Intel 选项 |
| | `omnimux_landing.hero.macIntelReq` | Intel 处理器 · macOS 13+ | Intel Processor · macOS 13+ | 系统要求 |
| | `omnimux_landing.hero.shuffle` | 🎲 换一批素材 | 🎲 Shuffle media | 换批按钮 |
| | `omnimux_landing.hero.poolTag` | 已固化 28 个素材 (展示 14 个) | 28 curated clips (14 visible) | 素材池标签 |
| | `omnimux_landing.hero.scrollCue` | 向下滚动探索画布 | Scroll down to explore canvas | 滚动提示 |
| **Screen 2 DAG** | `omnimux_landing.dag.sidebar.newChat` | 新会话 | New Chat | 侧边栏按钮 |
| | `omnimux_landing.dag.sidebar.newProject` | 新建项目 | New Project | 侧边栏按钮 |
| | `omnimux_landing.dag.sidebar.projects` | 工作区 (PROJECTS) | WORKSPACES (PROJECTS) | 分组标 |
| | `omnimux_landing.dag.scenes.ugcTalk.title` | UGC 真人口播带货视频 | UGC Humanoid E-commerce Video | 场景 1 |
| | `omnimux_landing.dag.scenes.skincareUgc.title` | 护肤品 UGC 真人讲解视频 | Skincare UGC Explainer Video | 场景 2 |
| | `omnimux_landing.dag.scenes.viralRecreate.title` | 爆款视频复刻 | Viral Video Re-creation | 场景 3 |
| | `omnimux_landing.dag.scenes.keyframeVideo.title` | 关键帧视频 | Keyframe Commercial Video | 场景 4 |
| | `omnimux_landing.dag.copilot.inputPlaceholder` | 输入提示词 / 使用 Skill 分析或编辑视频内容... | Enter prompt / Use Skill to analyze or edit video content... | 输入框 |
| **Screen 3 Steps** | `omnimux_landing.steps.title` | 让社媒运营 | Automate Social Operations with | 步骤标题 |
| | `omnimux_landing.steps.titleHighlight` | 全AI自动化 | Full AI Pipelines | 高亮紫字 |
| | `omnimux_landing.steps.subtitle` | 从多账号接入、Agent 创作、自动发布、数据分析、资产管理，贯穿全流程的专业级社交媒体生产工作流。 | From multi-account connectivity, Agent creation, scheduled publishing, to deep analytics and asset vaults—a professional production workflow. | 步骤副标 |
| | `omnimux_landing.steps.items.0.title` | 多账号全球社媒矩阵直连 | Multi-Account Global Connectivity | 步骤 1 |
| | `omnimux_landing.steps.items.1.title` | 脚本、分镜与音视频批量生成 | Multimodal Script & Video Synthesis | 步骤 2 |
| | `omnimux_landing.steps.items.2.title` | 多平台一键排期与智能调度 | Omnichannel Automated Publishing | 步骤 3 |
| | `omnimux_landing.steps.items.3.title` | 跨渠道实时数据分析与回流 | Real-time Metrics & Insights | 步骤 4 |
| | `omnimux_landing.steps.items.4.title` | AI Agent 自主生产协作闭环 | Autonomous Agent Team Execution | 步骤 5 |
| **Screen 4 Matrix** | `omnimux_landing.matrix.title` | 覆盖主流 | Covering the Mainstream | 矩阵标题 |
| | `omnimux_landing.matrix.titleHighlight` | 社媒平台矩阵 | Social Media Matrix | 矩阵高亮 |
| | `omnimux_landing.matrix.desc` | 一站式接入全球主流社交媒体平台，将账号管理、内容创作与数据洞察无缝集成，让全链路运营效率倍增。 | One API to connect global social networks. Unify account orchestration, content creation, and real-time analytics to amplify operating efficiency. | 矩阵说明 |
| **Screen 5 Plaza** | `omnimux_landing.plaza.title` | 插件广场 按需使用 | Plugin Plaza, Use on Demand | 广场标题 |
| | `omnimux_landing.plaza.sub` | 安装来自各行业从业者的专家、技能、插件与连接器，或创建你自己的专属能力。你使用得越多，它越懂得你的创作方式。 | Install experts, skills, plugins, and connectors built by industry practitioners, or build your own bespoke capabilities. The more you use it, the better it understands your workflow. | 广场副标 |
| | `omnimux_landing.plaza.cards.expert.name` | 商业视频操盘手 | Video Ad Strategist | 卡片 1 |
| | `omnimux_landing.plaza.cards.skill.name` | 爆款黄金前3秒导演 | Viral Hook Director | 卡片 2 |
| | `omnimux_landing.plaza.cards.plugin.name` | 社媒矩阵多发助手 | Matrix Multi-Publisher | 卡片 3 |
| | `omnimux_landing.plaza.cards.connector.name` | 本地资产与工作流连接器 | Local Vault Connector | 卡片 4 |
| **Screen 6 CTA** | `omnimux_landing.cta.downloadTitle` | 下载 OmniMux · 属于你的\n本地 Agent 社媒运营团队 | Download OmniMux · Your Local\nAgent Social Media Team | 下闭卡主标 |
| | `omnimux_landing.cta.downloadTitleLine1` | 下载 OmniMux · 属于你的 | Download OmniMux · Your Local | 下闭卡主标第 1 行 |
| | `omnimux_landing.cta.downloadTitleLine2` | 本地 Agent 社媒运营团队 | Agent Social Media Team | 下闭卡主标第 2 行 |
| | `omnimux_landing.cta.downloadSub` | 立即安装，连接本地文件，5 分钟搭建你的自动化运营Agent | Install now, connect local files, and build your automated social operations Agent in 5 minutes. | 下闭卡副标 |
| | `omnimux_landing.cta.payPerCall` | 按调用付费 · 无需订阅 · 即刻开始 | Pay per call · No subscription · Start instantly | 计费提示 |
| **Footer** | `omnimux_landing.footer.tagline` | TikTok营销内容自动化 | TikTok marketing content automation | 官方口径 |

---

## 三、国际化覆盖说明
全量 145 个 key 已同步至工程全部 7 门语言配置中：
- `web/src/i18n/locales/zh.json` (简体中文 - 官方基准)
- `web/src/i18n/locales/en.json` (英语 - 官方源文)
- `web/src/i18n/locales/zh-TW.json` (繁体中文)
- `web/src/i18n/locales/ja.json` (日文)
- `web/src/i18n/locales/ru.json` (俄文)
- `web/src/i18n/locales/vi.json` (越南文)
- `web/src/i18n/locales/fr.json` (法文)
并通过 `web/src/i18n/static-keys.ts` 完成静态提取登记。
