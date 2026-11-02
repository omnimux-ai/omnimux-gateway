# Prompt 五维体系

> 位置：`server/skills/image/image-creation/references/prompt-dimensions.md`
> 用途：图片生成的五维 prompt 工程框架，指导 Agent 构造高质量提示词
> 来源：适配自 baoyu-cover-image 五维体系 + Gxgen 实践

---

## 概览

每张图片可通过 5 个正交维度精确控制视觉效果。Agent 根据图片类型选择默认组合，再按用户需求微调。

| 维度 | 控制 | 默认值 |
|------|------|--------|
| **Type** | 构图与信息结构 | conceptual |
| **Palette** | 色彩方案 | auto（按内容推断） |
| **Rendering** | 渲染风格 | digital |
| **Text** | 文字层级 | none |
| **Mood** | 情绪氛围 | balanced |

---

## Dimension 1: Type（构图类型）

决定图片的视觉构图和信息呈现方式。

| Type | 描述 | 最佳场景 | Prompt 关键词 |
|------|------|---------|--------------|
| `hero` | 主体居中，视觉冲击力强 | 产品展示、人物特写、品牌宣传 | `centered composition, hero shot, dramatic focus on [subject]` |
| `conceptual` | 概念可视化，隐喻表达 | 科技、商业、抽象主题 | `conceptual illustration, visual metaphor, abstract representation of [concept]` |
| `typography` | 文字为主角，图形辅助 | 标语海报、金句卡片 | `typography-focused, text as visual element, decorative lettering` |
| `metaphor` | 视觉隐喻，用具象表达抽象 | 社论配图、深度内容 | `visual metaphor, symbolic imagery, [concrete thing] representing [abstract concept]` |
| `scene` | 场景叙事，环境氛围 | 生活方式、旅行、故事 | `scene composition, environmental storytelling, atmospheric setting` |
| `minimal` | 极简留白，单一焦点 | 高端品牌、简洁封面 | `minimalist, clean negative space, single focal point, elegant simplicity` |

---

## Dimension 2: Palette（色彩方案）

控制整体色调和配色风格。

| Palette | 描述 | 色彩特征 | Prompt 关键词 |
|---------|------|---------|--------------|
| `warm` | 温暖舒适 | 橙、琥珀、奶油色 | `warm color palette, amber and cream tones, cozy warm lighting` |
| `elegant` | 高级质感 | 金、深蓝、米白 | `elegant palette, gold accents, navy and cream, sophisticated colors` |
| `cool` | 冷静理性 | 蓝、灰、银 | `cool blue-gray palette, steel and ice tones, analytical color scheme` |
| `dark` | 深沉神秘 | 深色底、霓虹点缀 | `dark background, moody palette, deep tones with subtle highlights` |
| `earth` | 自然有机 | 棕、绿、赭石 | `earth tones, natural palette, olive green and warm brown` |
| `vivid` | 鲜艳活泼 | 高饱和度、多色 | `vivid saturated colors, bold and vibrant, high-contrast color blocking` |
| `pastel` | 柔和梦幻 | 低饱和度、粉色系 | `pastel palette, soft muted colors, dreamy pink and lavender` |
| `mono` | 单色系 | 单色不同明度 | `monochromatic palette, single-hue variations, tonal depth` |
| `retro` | 复古怀旧 | 褪色、胶片感 | `retro color grading, faded vintage tones, 70s warm desaturated` |

---

## Dimension 3: Rendering（渲染风格）

决定视觉的质感和制作手法。

| Rendering | 描述 | Prompt 关键词 |
|-----------|------|--------------|
| `flat-vector` | 扁平矢量，现代设计 | `flat vector illustration, clean geometric shapes, no gradients, modern graphic design` |
| `hand-drawn` | 手绘质感，亲和力 | `hand-drawn illustration, sketch-like quality, organic brush strokes, artistic imperfections` |
| `painterly` | 绘画艺术感 | `painterly style, visible brush strokes, oil painting texture, artistic rendering` |
| `digital` | 数字渲染，精致光影 | `digital illustration, polished rendering, smooth gradients, professional CGI quality` |
| `pixel` | 像素艺术，复古游戏 | `pixel art style, 8-bit aesthetic, chunky pixels, retro game graphics` |
| `chalk` | 粉笔画/黑板风 | `chalk drawing on blackboard, hand-lettered, dusty texture, educational aesthetic` |

---

## Dimension 4: Text（文字层级）

控制图片中文字内容的存在方式。

| Text | 描述 | Prompt 关键词 |
|------|------|--------------|
| `none` | 纯视觉，无文字 | `no text, pure visual, clean image without any typography` |
| `title-only` | 仅主标题 | `with title text "[TITLE]", prominent headline` |
| `title-subtitle` | 标题 + 副标题 | `with title "[TITLE]" and subtitle "[SUB]", clear text hierarchy` |
| `text-rich` | 多层文字信息 | `information-rich, multiple text elements, title subtitle and tags` |

**注意**：当 `generateImage` 的 `includeTextInImage` 为 false 时，强制使用 `none`。

---

## Dimension 5: Mood（情绪氛围）

控制视觉的情绪张力和对比度。

| Mood | 描述 | Prompt 关键词 |
|------|------|--------------|
| `subtle` | 低对比、柔和、内敛 | `subtle mood, low contrast, soft and understated, gentle visual tone` |
| `balanced` | 中等对比、稳重 | `balanced composition, medium contrast, harmonious visual weight` |
| `bold` | 高对比、强烈、冲击 | `bold dramatic mood, high contrast, strong visual impact, commanding presence` |

---

## 按图片类型的推荐组合

Agent 根据分类结果选择默认维度组合，用户有明确要求时覆盖。

### 封面图 / Cover Image

| 场景 | Type | Palette | Rendering | Text | Mood |
|------|------|---------|-----------|------|------|
| 技术/教程 | conceptual | cool | flat-vector | title-only | balanced |
| 品牌/营销 | hero | vivid | digital | title-subtitle | bold |
| 生活方式 | scene | warm | painterly | title-only | subtle |
| 高端/简约 | minimal | mono | digital | none | subtle |

### 海报 / Poster

| 场景 | Type | Palette | Rendering | Text | Mood |
|------|------|---------|-----------|------|------|
| 活动海报 | hero | vivid | digital | text-rich | bold |
| 艺术海报 | metaphor | elegant | painterly | title-only | balanced |
| 促销海报 | typography | warm | flat-vector | text-rich | bold |

### 社交媒体配图 / Social Card

| 场景 | Type | Palette | Rendering | Text | Mood |
|------|------|---------|-----------|------|------|
| 知识分享 | conceptual | cool | flat-vector | title-subtitle | balanced |
| 情绪共鸣 | scene | pastel | hand-drawn | title-only | subtle |
| 产品种草 | hero | warm | digital | title-only | balanced |

### 插画 / Illustration

| 场景 | Type | Palette | Rendering | Text | Mood |
|------|------|---------|-----------|------|------|
| 文章配图 | conceptual | auto | flat-vector | none | balanced |
| 叙事插图 | scene | earth | painterly | none | balanced |
| 装饰性 | minimal | pastel | hand-drawn | none | subtle |

---

## Prompt 组装模板

当 Agent 确定五个维度后，按以下模板组装最终 prompt：

```
[Rendering style] illustration of [Subject/Concept described by Type].
[Palette description] color scheme.
[Mood level] visual tone.
[Text instructions if not "none"].
[Additional user-specific requirements].
```

**示例**（技术封面图）：

```
Flat vector illustration of an abstract conceptual representation of AI neural networks.
Cool blue-gray palette with steel and ice tones.
Balanced composition with medium contrast.
With title text "Deep Learning Fundamentals", prominent headline.
Clean modern design, no realistic human faces.
```
