# OmniMux Design System Specification (DESIGN.md)

> **版本**: 2.3.0 (Violet Budget)
> **视觉对标**: [MiniMax Design System (design.minimax.io)](https://design.minimax.io/) × Modern AI Infrastructure
> **同级研究**: [Pippit AI 落地页拆解](docs/research/pippit-landing-analysis-2026-09-12.md)（2026-09-12，同口径实测）
> **核心基准**: 图 1（深色 Studio）与 图 3（浅色 Studio）实机界面
> **色彩基底**: **黑白中性控制台（Monochrome Ink/Paper）+ 极光电紫与皇家靛蓝（#7961f2 / #6757e7）专属 AI 能量流**
> **消费对象**: AI Agents (Cursor, Claude Code, Google Stitch, DeepSeek Harness), 前端开发工程师, UI/UX 设计师
>
> **2.2.0 变更摘要**：① §3.1 字体栈修正为**实现真源**（Geist 家族），删除从未落地的 Plus Jakarta Sans / Newsreader；② 新增 §3.4 字重阶梯、§3.5 字距阶梯；③ §3.2/§3.3 拆为**营销字阶**与**界面模拟字阶**两层；④ 新增 §5.2 间距系统、§5.3 圆角阶梯、§5.4 中性灰阶梯、§5.5 容器与断点；⑤ §6 阴影从 5 档收敛为 3 档；⑥ 新增 §4.3 层级策略、§4.5 导航外框规格；⑦ §7 新增"尺度纪律"与"禁止就地新造数值"铁律。
>
> **2.3.0 变更摘要**：新增 §2.2 品牌紫预算与白名单（对标 Pippit 同口径实测），门禁新增 `VIOLET_CTA_CSS`，拦截手写 CSS 里按钮类选择器的实心紫背景。
>
> **2.2.1 变更摘要**：§2.1 由「复制一份 token 清单」改为「只约定类别 + 指向实现真源」——公开面颜色的取值真源是 `landing.css` 的 `.omnimux-landing` 变量块。附带把公开面 CSS 的 107 处颜色字面量全部收敛到 token（残留 0 处），并在门禁加 `MALFORMED_VAR_CSS` 规则与 token 完整性断言。

---

## 1. Visual Theme & Atmosphere（视觉主题与氛围）

产品定位与文案以 [品牌规范](docs/brand/README.md) 为准；本文件定义视觉系统，不以素材输入数量或 SKU 数据模型约束页面叙事。

### 1.1 品牌设计哲学
OmniMux 定位于**属于你的内容电商团队**（AI 内容电商系统：涵盖商品与角色生成、参考视频拆解复刻与桌面工作台），其视觉设计语言承袭了 **MiniMax 现代 AI 基础设施** 的设计哲学：**极致通透的工程美学（Hyper-Precision Engineering Aesthetics）与高维电光质感**。

从旧版高刺激、容易产生视觉疲劳的荧光绿（Neon Green），蜕变为兼具**深度智慧、稳定感与高端科技活力**的 **Electric Violet（极光紫）与 Royal Indigo（皇家靛蓝: `#6757e7` / `#7961f2`）** 体系。整体呈现出如顶级实验室与精密暗房调色台般冷静、精密、高对比但克制优雅的视觉张力。

### 1.2 核心设计哲理：色彩职责三层正交（Role Orthogonality）
工作台必须严格区分“用户的意志控制”与“AI 机器的智能流转”，杜绝色彩通胀与视觉疲劳：
```
┌────────────────────────────────────────────────────────────────────────┐
│ 1. 结构与容器层 (Structural & Surfaces)                                 │
│    黑 / 白 / 石板中性灰                                                 │
│    作用：建立极高沉浸感、无色彩污染的“暗房画廊 / 创作画框”                 │
├────────────────────────────────────────────────────────────────────────┤
│ 2. 控制与物理操作层 (Control & Action)                                  │
│    黑白高反差墨水键 (Ink & Paper CTA)                                   │
│    作用：代表用户坚定的指令下发（执行/发送/提交），杜绝彩色大按钮滥用     │
├────────────────────────────────────────────────────────────────────────┤
│ 3. AI 智能与能量管道层 (AI Intelligence & Energy Flow)                  │
│    极光电紫 (Electric Violet & Royal Indigo: #6757e7 / #7961f2)         │
│    作用：代表 AI 算力在节点间的流动注入（Beam）、Logo 图腾、高阶算力标记 │
└────────────────────────────────────────────────────────────────────────┘
```

#### 为什么主操作（“执行”、“发送”）坚决不用紫色？
1. **语义纯粹性**：黑白对比代表“物理控制台与用户的意志输入”；紫色是“系统算力正在执行与流动”的专用色。用户点击执行是输入，AI 流动是反馈。
2. **画廊与暗房原则**：创作工作台的核心产物是多模态媒体（视频、图片、动画）。主界面必须是中性画框，避免高饱和彩色按钮抢夺生成内容的注意力。
3. **抗疲劳与工程感**：高频长时间操作下，单色按钮（黑白）认知负荷最低，极具工业级确定感（WCAG AAA 级高对比度）。

> **外部验证**：竞品 Pippit AI（pippit.ai）在主 CTA 上独立采用了同一策略——白底黑字胶囊、紫色只给产品内动作（Generate / TRY IT NOW）。本条是该哲学跨产品成立的佐证，不是可选项。

### 1.3 双主题设计标准（Dark & Light Dual Studio）
OmniMux 网站（公开页面、工作台与网关控制台）固定使用 **Dark（深色）主题**，不提供浅色或跟随系统的切换入口，不读取旧明暗偏好。桌面端等其他独立产品的双主题规范不受本次网站范围变更影响；下列浅色色值仅保留供这些独立界面参考：
- **Dark Studio（暗黑工作室模式，对标图 1）**：
  - 画布底色：纯中性深曜石黑 `#09090B` / `#0A0A0B`，搭配低对比微白点阵网格（`rgba(255,255,255,0.07)`）；
  - 容器卡片：微光半透纯深石墨灰卡片 `#141416`（无冷蓝偏色），配 `1px solid rgba(255,255,255,0.08)` 微光边；
  - 核心操作（执行/发送）：**纯白实心底 + 纯黑字/箭头**，提供极致清晰的对比度（> 18:1）；
  - AI 能量流：**亮紫电光流束（Beam `#7961F2` / `#9A88FA`）**，在黑底下呈现极光发光感。
- **Light Studio（明亮工作室模式，对标图 3）**：
  - 画布底色：纯净高明度纸白 `#FFFFFF` / `#F8F8F9`，搭配低对比暗灰点阵（`rgba(0,0,0,0.05)`）；
  - 容器卡片：纯白高洁卡片 `#FFFFFF`，配柔和暗灰分界线 `rgba(0,0,0,0.08)`；
  - 核心操作（执行/发送）：**纯黑实心底 + 纯白字/箭头（Ink CTA）**，稳重、硬朗、无杂色干扰；
  - **媒体预览（视频节点）特例**：**在浅色主题下依然保持深暗背景（`#11141D`）**，保持视听暗房环境，确保视频色彩真实还原；
  - AI 能量流：**深邃皇家靛紫（Beam `#6757E7` / `#5644D6`）**，在亮底上拥有高饱和轮廓，拒绝冲淡。

**当前角色落地页（公开营销面）**：主画布为深色中性底，主按钮为白底深字，紫色用于品牌与少量强调。取值真源为 [landing.css](web/src/features/omnimux-landing/landing.css) 的 `.omnimux-landing` 变量块，布局为 [influencer-landing.css](web/src/features/omnimux-landing/influencer-landing.css)。这是独立公开页面，不因控制台切换浅色主题而自动改成暖白；不得从旧研究恢复暖纸白首页。

### 1.4 核心视觉特征关键词
- `Precise Matrix（精密点阵）`：20px × 20px 径向微点网格，界定专业工程坐标感。
- `Dual-Layer Glass（双层微光拟态）`：`backdrop-filter: blur(20px)` 配合 `1px` 细微白光边框。
- `Ink & Paper CTA（黑白高反差主操作）`：执行与发送按钮坚决使用黑白反差，不涂紫色。
- `Electric Violet Energy（极光电紫能量流）`：`#6757E7` / `#7961F2` 专属赋予智能连线、粒子光束与品牌锁扣。
- `Pill & Rounded Rect（胶囊与超圆角）`：胶囊形 CTA 与 16px~20px 柔和容器圆角（阶梯见 §5.3）。
- `Scale Discipline（尺度纪律）`：字号 / 字重 / 圆角 / 阴影 / 间距 / 文本灰只能取自 §3.2、§3.4、§5.2、§5.3、§5.4、§6 的白名单，**禁止就地新造数值**。

---

## 2. Color Palette & Roles（调色板与双模角色）

```css
/* ================= 浅色工作室模式 (Light Studio) ================= */
:root {
  /* 品牌 Logo 与 IP */
  --logo-bg-vibrant: #7961f2;      /* 极光紫背景 */
  --logo-bg-lavender: #b8b7ff;     /* 柔和香芋紫 */
  --logo-mascot-charcoal: #0c0f17; /* 灵动曜石黑 IP */
  --logo-mascot-white: #ffffff;

  /* 核心品牌色 (用于流光、Logo、品牌标签，不用作主按钮) */
  --brand-indigo: #6757e7;         /* 主品牌色 */
  --brand-indigo-hover: #5644d6;   /* 悬浮深态 */
  --brand-purple: #7961f2;         /* 辅助亮紫 */
  --brand-purple-light: #9a88fa;   /* 浅紫高光 */
  --brand-purple-soft: #f0eefc;    /* 紫色浅底 */
  --brand-purple-glow: rgba(103, 87, 231, 0.20); /* 能量微光晕 */

  /* 中性底色与表面 (Light) */
  --bg-app: #f8f8f9;               /* 全局底色 */
  --surface-white: #ffffff;        /* 纯白卡片表面 */
  --surface-soft: #f2f3f5;         /* 灰阶浅底 */
  --surface-media-dark: #11141d;   /* 视频/图像媒体暗底 (浅色模式下特例保留) */
  --surface-ink: #0c0f17;          /* 纯黑操作按钮底 (Ink CTA) */
  --surface-ink-hover: #1e1f29;    /* 黑按钮悬浮态 */

  /* 文本色阶 (Light) —— 只有 3 档，见 §5.4 */
  --text-primary: #0c0f17;         /* 一级正文/大标题 */
  --text-secondary: #45515e;       /* 二级辅助说明 */
  --text-muted: rgba(17, 17, 22, 0.58); /* 三级弱化说明 */
  --text-disabled: #a8aab2;
  --text-on-dark: #ffffff;         /* 黑键/暗卡片纯白字 */

  /* 边框与微光 (Light) */
  --border-light: rgba(0, 0, 0, 0.06);
  --border-subtle: #e5e7eb;
  --border-ink: rgba(0, 0, 0, 0.15);
  --dot-matrix: rgba(0, 0, 0, 0.05);

  /* AI 能量流动流光 (Light - 靛紫高饱和) */
  --beam-start: #6757e7;
  --beam-end: #5644d6;
  --beam-core: #7961f2;
  --beam-glow: rgba(103, 87, 231, 0.20);

  /* 语义反馈色 */
  --success: #16a34a;
  --warning: #d97706;
  --error: #dc2626;
  --info: #2563eb;
}

/* ================= 暗黑工作室模式 (Dark Studio) ================= */
[data-theme='dark'], .dark {
  /* 中性底色与表面 (Dark) —— 纯曜石黑与中性深石墨灰，严禁冷蓝偏色 */
  --bg-app: #09090b;               /* 纯深曜石底色 (#0a0a0b) */
  --surface-white: #141416;        /* 替换为纯深石墨灰表面 (纯正中性) */
  --surface-soft: #18181b;         /* 抬升表面 */
  --surface-media-dark: #080c14;   /* 极暗媒体底色 */
  --surface-ink: #ffffff;          /* 纯白操作按钮底 (Paper CTA) */
  --surface-ink-hover: #f0f0f2;    /* 白按钮悬浮态 */

  /* 文本色阶 (Dark) —— 只有 3 档，见 §5.4 */
  --text-primary: #ffffff;
  --text-secondary: rgba(255, 255, 255, 0.72);
  --text-muted: rgba(255, 255, 255, 0.40);
  --text-disabled: rgba(255, 255, 255, 0.25);
  --text-on-dark: #0c0f17;         /* 白按钮上的黑字 */

  /* 边框与微光 (Dark) */
  --border-light: rgba(255, 255, 255, 0.08); /* 1px 精密微光边框 */
  --border-subtle: rgba(255, 255, 255, 0.12);
  --border-ink: rgba(255, 255, 255, 0.15);
  --dot-matrix: rgba(255, 255, 255, 0.07);

  /* AI 能量流动流光 (Dark - 极光发光紫) */
  --beam-start: #7961f2;
  --beam-end: #9a88fa;
  --beam-core: #ffffff;
  --beam-glow: rgba(121, 97, 242, 0.35);

  /* 语义反馈色 (Dark) */
  --success: #4ade80;
  --warning: #fbbf24;
  --error: #f87171;
  --info: #3b82f6;
}
```

### 2.1 落地页营销色（`--omx-*`，公开面真源）

落地页自带一层营销 token。**取值真源是 `web/src/features/omnimux-landing/landing.css` 的 `.omnimux-landing` 变量块**（避免本文件与实现各存一份而漂移）；本文件只约定**类别与用途**：

| 类别 | 前缀 | 用途 |
| :--- | :--- | :--- |
| 表面 Surface | `--omx-bg*` / `--omx-surface-*` / `--omx-canvas-*` | 页面画布、抬升卡片、暗房媒体面、墨黑块 |
| 描边 Border | `--omx-border*` | 冷墨 alpha 阶梯，4 档 |
| 暗面上的白 On-dark | `--omx-on-dark-*` / `--omx-scrim*` | 媒体卡描边、文字、遮罩 |
| 品牌 Brand | `--omx-brand*` | 品牌色阶、浅底、渐变、品牌描边 |
| 语义 Semantic | `--omx-success*` / `--omx-error` | 成功 / 失败反馈 |
| 深度 Depth | `--shadow-sm` / `-md` / `-brand` | §6 的三档阴影 |

> 铁律：**公开面 CSS 不得出现颜色字面量**（`#hex` / `rgba()`），一律引用上述 token——除非是 CSS 注释里的说明文字、`mask-image` 的亮度掩码、或 alpha 通道的分量（如渐变起始位）。`guard-ui-design` 的 `OFF_*` 系列规则负责守住尺度，token 完整性由测试的 `public CSS tokens are all defined` 断言守住。

### 2.2 品牌紫预算与白名单（公开营销面）

紫色只表示「AI 正在干活」或「当屏唯一的动手操作」，其余一律黑白灰。预算按 1440×900 视口逐屏测量（色相 235–300°、饱和度 ≥35%、亮度 >0.2 的像素占比）：

| 预算 | 上限 | 参考 |
| :--- | :--- | :--- |
| 首屏 | **0.5%** | Pippit 首屏 0.21%（只有一个 Generate） |
| 其余每屏 | **1%** | Pippit 0–0.78% |
| 每屏实心紫按钮 | **最多 1 个**，且不能是转化 CTA（下载 / 注册 / 购买） | Pippit 页头、页脚转化 CTA 均为白底黑字 |

**白名单**（只有这些位置可以用紫色）：

| 用途 | 写法 |
| :--- | :--- |
| Logo 与品牌锁扣 | 原样 |
| 首屏唯一的「动手」动作键（发送 / 生成） | 浅紫底 `#9A88FA` + 黑色图标或文字（对比度约 7:1，白字不达标）；CSS 同行加 `exempt-ui` 注释 |
| AI 画布节点连线（Beam） | 线与端点 |
| 「生成中」扫光 | 低透明度扫光，静止态不留紫 |
| 图表里唯一的高亮数据 | 只给一根柱或一个数据点，其余中性 |

**禁止用紫色的位置**：主按钮与转化 CTA、大面积背景光晕、选中态 / Tab / 激活步骤（改用白色或白底）、装饰图标、区块标签、提示卡与确认卡底色、头像、页面底色与文字灰（底色必须是 §1.3 纯中性灰阶，不得带紫调）。

> 依据：2026-10-27 同口径实测，首页修改前首屏 7.43%、全页每屏平均 1.37%；Pippit 首屏 0.21%、平均约 0.29%。门禁 `VIOLET_CTA_CSS` 拦截按钮类 CSS 选择器上的实心紫背景；面积预算靠逐屏测量验收，门禁不测面积。

---

## 3. Typography Rules（排版规则）

### 3.1 字体栈族（Font Families）

**真源（与 `web/` 实现一致；禁止声明未落地的字体）**：

| 角色 | 字体栈 | 说明 |
| :--- | :--- | :--- |
| **Display Serif（大标点睛）** | `'Instrument Serif', 'Noto Serif SC', 'Noto Serif TC', 'Noto Serif JP', 'Songti SC', serif` | 自托管开源衬线族（SIL OFL 1.1），用于 Hero/CTA 大标与品牌点睛 |
| **Display & UI（默认）** | `'Geist Variable', 'PingFang SC', sans-serif` | 自托管可变字体，全站唯一无衬线族 |
| **Mono（数据/代码）** | `'Geist Mono Variable', monospace` | 时间戳、Token 计数、代码块 |

**衬线点睛（已启用）**：参考 Pippit AI 现代社论风格，引入自托管开源免费商用 `Instrument Serif`（400 / Italic，SIL OFL 1.1），严格限制在显示层：Hero 主标题（`--font-display-serif`）、卡片商品标签 Italic 与底部 CTA 大标。正文与常规 UI 保持统一现代无衬线体。

> ⚠️ **历史修正（2.2.0）**：2.1.0 曾声明 `"Plus Jakarta Sans"` 与 `"Newsreader"`。经 2026-09-12 全仓核查，二者在 `web/src` 内**零命中**，实际渲染的是 `Geist Variable`（217 处）/ `Geist Mono Variable`（29 处）。本表已改为实现真源。

### 3.2 字阶规范表（Type Scale Hierarchy）

**营销字阶**——公开营销面（落地页、公开组件、灵感社区）的正文排版白名单。字号只能取自此表。

| 级别 (Token) | 桌面 | ≤640px | 字重 | 行高 | 字距 | 典型应用场景 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `display-hero` | 64px | 32px | 700 | 1.12（≤2 行）/ 1.40（≥3 行） | -0.035em | 落地页 Hero 主标题 |
| `display-lg` | 44px | 28px | 600 | 1.25 | -0.025em | 区块一级大标题（H2） |
| `heading-xl` | 32px | 24px | 600 | 1.30 | -0.02em | 对话框标题、阶段大模块 |
| `heading-lg` | 24px | 20px | 600 | 1.35 | -0.015em | 卡片组标题、品牌名 |
| `heading-md` | 20px | 18px | 600 | 1.40 | -0.01em | 列表分组标题、次级小标题 |
| `body-lg` | 18px | 16px | 400 | 1.55 | 0em | 引导句、副标题 |
| `body-base` | 16px | 16px | 400 | 1.60 | 0em | 营销正文默认 |
| `body-sm` | 14px | 14px | 400 | 1.50 | 0.005em | 辅助说明、按钮文字 |
| `caption` | 12px | 12px | 500 | 1.40 | 0.01em | 状态标签、时间戳、角标 |

**公开面字号白名单**（与 `scripts/ops/guard-ui-design-rules.mjs` 的 `TYPE_SCALE` 必须逐值一致）：

```
64, 44, 32, 28, 24, 20, 18, 16, 14, 12, 11
```

> 结构说明：`28` 是 `display-lg` 与 `display-hero` 的移动档；`11` 属 §3.3 微字阶，**公开营销正文一律不得使用**。

### 3.3 界面模拟字阶（Mockup & Console Micro-Type）

公开面常把「产品界面截图」用 DOM 渲染出来（`.omx-m-*`、`.m-*`、`omx-dag-node`、`studio-bar`），需要用**密集微排版**才像真实 App；网关控制台同样刻意采用紧凑排版。这层不适用 §3.2。

| 档位 | 用途 | 约束 |
| :--- | :--- | :--- |
| 13px | 模拟图主文本、控制台紧凑正文 | 模拟图/控制台内可用 |
| 12px | 模拟图标签、次级文本 | 同上 |
| 11px | 模拟图角标、DAG 节点标签、波形时间 | 同上 |
| 10px | 徽标、状态胶囊 | **仅限单行 ≤6 字** |

**边界铁律**：微字阶**只允许出现在模拟图容器与网关控制台内**。公开营销正文（标题、段落、按钮、CTA、卡片描述）下限为 **12px**，不得援引本小节降级。

> **现状核对（2026-09-12 实测）**：落地页 11px 共 31 处，其中 25 处在模拟图选择器（`.m-*` / `.omx-m-*`）内、5 处在 `omx-dag-node` 等模拟元件内，营销正文 0 处——符合本节约束。

**落地页阅读基线**：营销正文 16px / 400 / 1.6；简短辅助说明 14px / 400 / 1.5。落地页通过 `--omx-type-body`、`--omx-type-body-leading`、`--omx-type-support`、`--omx-type-support-leading` 复用这些角色。描述使用 `text-wrap: pretty`，标题使用 `text-wrap: balance`。三行及以上标题行高不得低于 1.4。

### 3.4 字重阶梯（Font Weights）

**只允许 4 档**：

| 值 | 名称 | 用途 |
| :--- | :--- | :--- |
| 400 | Regular | 正文、描述 |
| 500 | Medium | 标签、次级按钮、导航项 |
| 600 | SemiBold | 标题、主按钮、强调 |
| 700 | Bold | 仅 `display-hero` |

**禁止**中间值字重（550 / 650 / 800 等）。非可变字体会在这些值上静默回退到最近档位，造成同一视觉角色在不同环境渲染不一致。

> **气质取向**：本表最大字重止于 700，且只有 Hero 一档使用。参考实现 Pippit AI 走得更远——其 104px Hero 用 400、40px 区块标题用 **300**，靠**字号**建立层级而非字重。本系统保留 600/700 以维持 B 端工业确定感，但**禁止**用 800+ 制造层级。

### 3.5 字距阶梯（Letter Spacing）

字距一律用 **`em`**（禁止 `px`），并随字号单调收紧：

| 字号区间 | 字距 |
| :--- | :--- |
| ≥44px | `-0.035em` |
| 24–32px | `-0.02em` |
| 18–20px | `-0.01em` |
| 16–17px | `0` |
| 12–14px | `0.005em` |
| 全大写标签 | `0.04em` |

---

## 4. Component Stylings（核心组件样式规范）

### 4.1 按钮系统（Buttons）

```css
/* 1. Primary Button (Dark/Black Pill CTA - 坚决黑白墨水化，不涂紫色) */
.btn-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 40px;
  padding: 0 22px;
  border-radius: 999px;
  background: var(--surface-ink);
  color: var(--text-on-dark);
  font-size: 14px;
  font-weight: 600;
  border: 1px solid var(--border-ink);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.btn-primary:hover {
  background: var(--surface-ink-hover);
  transform: translateY(-1.5px);
  box-shadow: var(--shadow-md);
}

/* 2. Round Send Button (输入框圆形发送按键) */
.btn-send {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--surface-ink);
  color: var(--text-on-dark);
  border: none;
  cursor: pointer;
  transition: transform 0.15s ease, opacity 0.15s ease;
}
.btn-send:hover {
  transform: scale(1.05);
}

/* 3. Brand Electric Violet Button (品牌展示/算力套餐胶囊，非日常主操作) */
.btn-brand {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 36px;
  padding: 0 18px;
  border-radius: 999px;
  background: var(--gradient-brand-primary, #7961f2);
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  border: none;
  box-shadow: var(--shadow-brand);
  cursor: pointer;
  transition: all 0.2s ease;
}

/* 4. Ghost / Secondary Button (次级操作键) */
.btn-secondary {
  height: 36px;
  padding: 0 16px;
  border-radius: 999px;
  background: var(--surface-soft);
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 500;
  border: 1px solid var(--border-light);
  transition: all 0.15s ease;
}
```

**按钮尺寸阶梯**：`36px`（紧凑/工具栏）、`40px`（主 CTA 默认）、`48px`（落地页 Hero CTA）、`52px`（落地页页脚大 CTA）。圆角一律 `999px` 胶囊。

### 4.2 卡片容器（Cards & Surfaces）

```css
/* 工作台节点卡片 (支持 Dark/Light 自动自适应) */
.card-node {
  background: var(--surface-white);
  border-radius: 16px;                    /* --radius-card，见 §5.3 */
  border: 1px solid var(--border-light);
  box-shadow: var(--shadow-sm);
  padding: 20px;
  transition: box-shadow 0.2s ease, transform 0.2s ease;
}
.card-node:hover {
  box-shadow: var(--shadow-md);
}

/* 视听媒体节点卡片 (浅色模式下依然保持暗黑底色) */
.card-media-studio {
  background: var(--surface-media-dark);
  border-radius: 20px;                    /* --radius-media，见 §5.3 */
  border: 1px solid rgba(255, 255, 255, 0.10);
  box-shadow: var(--shadow-md);
  overflow: hidden;
  position: relative;
}
```

### 4.3 层级策略（Elevation Strategy）

层级优先级：**1px 半透明描边 → 面层明度差 → 阴影**。阴影是最后手段，不是首选。

```css
/* 抬升靠描边与面色，不靠投影 */
.surface-raised {
  background: var(--surface-white);
  border: 1px solid var(--border-light);   /* Dark: rgba(255,255,255,.08) */
}
```

> 参考实现 Pippit AI 全页**只用了 2 种阴影**，层级完全由 `1px rgba(255,255,255,.04/.08/.12)` 描边 + 面层明度差（`#000` → `#0D0D0D` → `#1C1C1C`）承担。本系统上限为 3 档（见 §6）。

### 4.4 玻璃拟态（Dual-Layer Glass）

```css
.glass-surface {
  background: var(--color-surface, rgba(255, 255, 255, 0.85));
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.10);
}
```

### 4.5 导航外框规格（Header / Nav Chrome）

站点外框（`landing-header` / `public-header` / `landing-footer`）不适用 §3.2 内容字阶，但必须遵守本节：

| 属性 | 值 |
| :--- | :--- |
| 栏高 | 桌面 64px / 移动 56px |
| 导航项字号 | **14px / 500**（禁止 15px 及以上的散值） |
| 导航项内边距 | `0 20px` |
| 导航胶囊 | 圆角 `999px`、背景 `hsla(0,0%,60%,.16)`、`backdrop-filter: blur(20px)` |
| 悬浮 / 激活文字色 | `--text-primary`（Dark 下为 `#fff`） |
| 滚动收敛 | 收敛后胶囊化，缓动 `cubic-bezier(0.16, 1, 0.3, 1)`；宽度变化必须用可插值属性 |

当前角色落地页的顶栏高度为桌面 72px、窄屏 60px，导航文字为 14px；整幅品牌标识复用 [BrandWordmark](web/src/components/brand-wordmark.tsx)。该页面的实际外框规格以 [landing-header.tsx](web/src/features/omnimux-landing/components/landing-header.tsx) 为准；本节 64px/56px 是其他外框基线，不应用旧外框快照改回角色页面。

---

## 5. Layout Principles（布局与尺度系统）

### 5.1 像素点阵背景（MiniMax Signature Dot Matrix）

```css
/* 双模点阵背景 CSS 实现 */
.ambient-matrix-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: radial-gradient(circle, var(--dot-matrix) 0.75px, transparent 0.95px);
  background-position: 2px 2px;
  background-size: 20px 20px;
  mask-image: radial-gradient(ellipse 78% 68% at 50% 48%, #000 28%, transparent 84%);
  -webkit-mask-image: radial-gradient(ellipse 78% 68% at 50% 48%, #000 28%, transparent 84%);
}
```

### 5.2 间距系统（Spacing Scale）

**基底 8px 主网格**。所有 `gap` / `padding` / `margin` 只能取下列值：

| Token | 值 | 典型用途 |
| :--- | :--- | :--- |
| `--space-1` | 4px | 图标与文字、紧邻元素 |
| `--space-2` | 8px | **默认 gap**（最常用） |
| `--space-3` | 12px | 卡片内元素间距 |
| `--space-4` | 16px | 卡片内边距（小） |
| `--space-6` | 24px | 卡片内边距（大） |
| `--space-8` | 32px | 组与组之间 |
| `--space-10` | 40px | 区块内边距（32 与 48 之间的中间档） |
| `--space-12` | 48px | 区块内部段落间距 |
| `--space-16` | 64px | 区块内边距（移动） |
| `--space-20` | 80px | 区块内边距（桌面） |
| `--space-25` | 100px | 主交互与下一模块之间 |
| `--space-30` | 120px | 大区块之间 |

**禁止非网格值**（5px、7px、9px、13px、17px、18px、22px 等）。确需 2px 级微调时只用 `2px`（描边 / 位移补偿），并在同行注明原因。

**Hero 垂直节奏**（参考实现实测，比例关系可直接沿用）：

```
80px  顶部留白
 27px 眉标胶囊（pad 4px 10px / radius 16px）
  8px ← 极紧
125px H1
 30px
182px 主交互块（composer / 搜索框 / CTA 组）
100px ← 大留白
     次模块
```

规律：**越近越紧，越远越松**（8 → 30 → 100）。

### 5.3 圆角阶梯（Radius Ladder）

**只允许下列语义圆角**：

| Token | 值 | 语义 |
| :--- | :--- | :--- |
| `--radius-chip` | 8px | 标签、chip 容器、内嵌小面 |
| `--radius-card` | 16px | 卡片、面板 |
| `--radius-media` | 20px | 媒体容器（视频 / 图片框） |
| `--radius-input` | 24px | 输入框、大容器 |
| `--radius-hero-input` | 36px | 落地页 Hero 主输入框（特例，仅此一处） |
| `--radius-pill` | 999px | 按钮、胶囊、徽标 |

**角色历史画廊特例（2026-10-10 用户指定）**：任务卡片外框与历史加载占位使用直角（0px）；历史网格的图片、多视角图与视频统一固定宽高比1.75578（2026-10-11用户要求1:1对齐，按多宽度实测更新），横纵间距 4px，无边框与阴影，媒体完整包含。达人历史固定使用网格，不提供列表/时间线切换，不读取旧显示模式偏好。历史网格依可用内容宽度以240px阶梯切换1至4列，左右8px内距；历史工作区最大1920px。媒体留白使用中性#222，操作区轻渐变在桌面仅悬停/聚焦揭示，触屏常显。卡内转视频为高32px、8px圆角、边距8px；按参考站实测及已确认候选，内距采用上6px、左右12px、下8px作为仅此动作的光学对齐例外（保留16px行高和透明边框），下载视觉在卡宽384px以上32px、其余18px，触屏保留44px命中区。网格占位与成品使用相同实测画幅，品牌配色保留。头像、其他媒体容器继续使用既有圆角。

**禁止** 2px / 4px / 6px / 7px / 10px / 11px / 12px / 14px / 18px / 28px 等散值；`50%` 仅用于正圆（头像、圆形图标键）。

**角色工作台分层特例**：工作台左右外壳与分组壳复用20px媒体圆角，内部托盘和灵感卡复用16px卡片圆角，夸张度卡复用8px小面圆角。仅工作台模式轨道采用实测10px圆角，内按钮8px，桌面轨道36px、手机44px；主生成按钮仍为40px黑白胶囊。所有未选中选项无可见描边，保留透明边占位以避免选中尺寸跳变；性别为透明中性方卡；角色类别与性别名称默认常显；年龄（age）、肤色（skin_tone）、瞳色（eye_color）和发色（hair_colour）采用88×88正方外框，紫色选中边包含上方76×48横版视觉与下方默认常显白字名称，外16px/内8px圆角复用既有令牌，默认透明底，悬停中性浅底，选中轻紫底；选中颜色在色块中央显示勾号，无悬停遮罩或位移；颜色名称优先单行，必要长译文按12px/16px字阶换行，保持固定框与完整可访问名称；年龄无照片时复用现有中性年龄图标且名称只出现一次，不新增人物图片。其余媒体、色块与文字选项采用方卡，复用历史整卡由黑10%至85%的渐变遮罩，名称以无底条白字仅悬停或键盘聚焦显示，选中本身不触发常显。触屏常显，窄屏鼠标仍遵循悬停规则。长译文允许卡片增高，不裁掉名称。分组壳采用主题前景5%叠色与4px内距，托盘8px内距/8px间距；整体表单采用1px内描边，不额外占布局宽度。此处局部规格不改变全站令牌或历史作品画幅。

### 5.4 中性灰阶梯（Neutral Ramp）

**文本灰只有 3 档**（对齐 §2.1 的 `--omx-text` / `--omx-text-muted` / `--omx-text-faint`）：

| 档 | Light | Dark | 用途 |
| :--- | :--- | :--- | :--- |
| 一级 | `#0c0f17` | `#ffffff` | 标题、正文主体 |
| 二级 | `#5a6578` | `rgba(255,255,255,.72)` | 描述、辅助说明 |
| 三级 | `#8a94a6` | `rgba(255,255,255,.40)` | 时间戳、弱化元信息 |

**禁止新增第 4 档灰**。需要一个"稍微淡一点"的灰时，改用透明度（`color-mix` 或 `opacity`），不要新十六进制值。

暗色底上的弱化白同样只有 3 档：`.40` / `.60` / `.80`。

### 5.5 容器宽度与断点

**内容容器**：当前角色落地页 `.inf-section` 最大宽度为 `1152px`，窄屏侧距 20px、640px 起侧距 48px，真源为 [influencer-landing.css](web/src/features/omnimux-landing/influencer-landing.css)。通用容器基线为 `1320px`、`760px` 正文窄栏与 `1440px` 站点外框，不用通用基线覆盖该页已确认布局。

**断点**（遵循 Tailwind 默认集，禁止另造）：

```
sm 640px · md 768px · lg 1024px · xl 1280px · 2xl 1536px
```

---

## 6. Depth & Elevation（深度与多层层级）

**只有 3 档阴影**（2.1.0 的 5 档收敛至此）：

```css
:root {
  /* 卡片默认：几乎不可见的贴地投影 */
  --shadow-sm: 0 1px 3px rgba(12, 15, 23, 0.06);
  /* 悬浮 / 抬升元素 */
  --shadow-md: 0 10px 24px -4px rgba(12, 15, 23, 0.18);
  /* 品牌能量光晕（仅限品牌色元素） */
  --shadow-brand: 0 10px 24px -4px rgba(103, 87, 231, 0.28);
}
```

**禁止**：负 spread 小于 -4px 的深长投影、双层叠加阴影、`rgba(0,0,0,.3)` 以上的重投影、以及任何第 4 种阴影值。

> **实测基线**：落地页曾同时存在 **13 种**阴影；参考实现全页仅 2 种。收敛到 3 档后，层级改由 §4.3 的描边 + 面色承担。

---

## 7. Do's and Don'ts（设计规范与铁律禁忌）

### ✅ 必须执行（Do's）
1. **主行动点黑白墨水化**：Primary CTA（“执行”、“保存”、“发送”）在 Light 下一律纯黑白字，在 Dark 下一律纯白黑字，绝对不用紫色。
2. **极光紫专职专责**：紫色仅用于节点贝塞尔连线流动动效（Beam）、品牌 Logo 锁扣和特定算力计划标签；公开营销面另按 §2.2 预算与白名单执行。
3. **视听暗房原则**：浅色模式下，视频播放卡片与首尾帧预览卡片必须保持暗色背景（`#11141D`），不得强行翻白。
4. **文字遵守 WCAG AAA 级高对比度**：确保长时间生产力操作不刺眼、不疲劳。
5. **尺度纪律**：字号取 §3.2 白名单、字重取 §3.4 四档、圆角取 §5.3 五档、阴影取 §6 三档、间距取 §5.2 八的倍数、文本灰取 §5.4 三档。需要"介于两档之间"的值时，先停下来——那是 §3.2 表需要补档的信号，不是就地造值的许可。
6. **层级优先用描边**：新增层次先试 `1px` 半透明描边与面层明度差，阴影是最后手段（§4.3）。

**历史图片转视频局部特例（2026-10-10 用户指定）**：仅历史图片卡片内侧右下角的“转为视频”快捷入口使用既有品牌紫底与白字，悬停/键盘焦点显示、触屏常显。该操作只加入参考图片、不提交生成；放大预览入口、重新生成及其他主操作仍保持黑白，不将此特例泛化到全站。

### ❌ 严厉禁忌（Don'ts）
1. **严禁在主按钮（Primary Button）上直接刷实心紫色**。
2. **严禁引入高饱和荧光绿（`#C8F135` / `#D4FF38`）**充当品牌色或连线色。
3. **严禁在同一界面混杂出现“紫 Logo + 蓝按钮 + 绿连线”的三元冲突**。
4. **严禁在浅色主题下将视听生成卡片漂白**，破坏暗房视听质感。
5. **严禁在本规范之外就地新造字号 / 字重 / 圆角 / 阴影 / 间距数值**；现有实现偏离本规范时改实现，不改规范——除非本规范本身有误，那时先修订本文件再改代码。
6. **严禁在公开营销正文使用 §3.3 的微字阶**（<12px）。
7. **严禁在深色模式下使用冷调偏蓝杂色**（如 `#151A26` / `#111520` 等发蓝冷石板色），深色模式必须使用纯正中性深石墨灰阶（Neutral Charcoal / Zinc，`#09090b` / `#141416` / `#18181b`）。

---

## 8. Agent & Visual-Dev Quick Reference（速查表）

| 用途 | Light 模式值 | Dark 模式值 |
| :--- | :--- | :--- |
| **主按钮底色** | `#0C0F17` (纯黑) | `#FFFFFF` (纯白) |
| **主按钮文字色** | `#FFFFFF` (纯白) | `#0C0F17` (纯黑) |
| **发送按钮底色** | `#0C0F17` (纯黑圆底) | `#FFFFFF` (纯白圆底) |
| **AI 连线粒子流 (Beam)** | `#6757E7` / `#5644D6` (皇家靛紫) | `#7961F2` / `#9A88FA` (极光亮紫发光) |
| **媒体卡片底色** | `#11141D` (保持暗黑) | `#080C14` (极暗石板) |
| **常规容器底色** | `#FFFFFF` (纯白) | `#141416` (纯深石墨灰，无偏蓝) |
| **字体族** | `'Geist Variable', 'PingFang SC', sans-serif` | 同左 |
| **正文** | 16px / 400 / 1.6 | 同左 |
| **卡片圆角 / 媒体圆角** | 16px / 20px | 同左 |
| **卡片阴影** | `0 1px 3px rgba(12,15,23,.06)` | 同左 |

### 8.1 落地页尺度自查清单

改动公开营销面前，逐条核对：

- [ ] 没有新增 §3.2 白名单外的字号（`64/44/32/28/24/20/18/16/14/12`；`11` 仅限 §3.3 模拟图）
- [ ] 没有 550 / 650 / 800 字重
- [ ] 没有 §5.3 之外的圆角
- [ ] 没有 §6 三档之外的阴影
- [ ] 没有 §5.2 网格外的间距（含 `gap`）
- [ ] 没有第 4 档文本灰
- [ ] 主 CTA 仍是黑白墨水，不是实心紫
- [ ] 紫色只出现在 §2.2 白名单位置；首屏 ≤0.5%、其余每屏 ≤1%
- [ ] 媒体卡在浅色下仍是暗底

```bash
# 门禁自查（全树回放，任何拒绝即失败）
node scripts/ops/guard-ui-design.test.mjs
```

---

**UI Designer** · 设计系统规范
**版本日期**: 2026-09-12 · **上一版**: 2.2.0 (Scale Convergence & Spec-Reality Rebase) (Dual-Mode Studio System & Role-Orthogonal Refresh)
