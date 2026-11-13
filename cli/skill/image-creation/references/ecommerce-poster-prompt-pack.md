# E-commerce Poster Prompt Pack

> 位置：`server/skills/image/image-creation/references/ecommerce-poster-prompt-pack.md`
> 用途：电商海报场景的结构化 prompt 模板与风格约束参考
> 加载方式：由 Agent 在识别到电商海报意图时通过 `skill_read` 按需加载

---

## 场景识别信号

当用户请求包含以下关键词或输入特征时，触发电商海报 prompt 模板：

- **意图关键词**：电商、海报、产品图、推广、banner、商品展示、店铺装修、主图、详情页、卖点图、促销海报
- **输入特征**：产品图 URL、产品描述、品牌名称、卖点文案、价格信息、目标平台（淘宝、天猫、京东、拼多多、小红书、抖音）

---

## 海报类型定义（Flexible Quantity）

Agent 根据用户意图和提供的产品信息，灵活选择以下类型组合。不强制固定数量，由用户意图决定生成几张。

| sceneType | 中文名 | 用途 | aspectRatio | mustPreserve |
|-----------|--------|------|-------------|--------------|
| hero | 主视觉 | 店铺首页 Banner、品牌首图、活动头图 | 3:4 / 16:9 | 产品主体外观、品牌色、核心卖点 |
| lifestyle | 生活方式 | 场景化展示，情感连接，使用氛围 | 3:4 / 1:1 | 产品外观；场景/搭配可创意 |
| detail | 细节特写 | 材质、工艺、功能点、纹理展示 | 1:1 | 严格复用产品材质、颜色、结构细节 |
| concept | 概念图 | 品牌理念、价值主张、愿景表达 | 16:9 / 3:4 | 无；允许基于品牌调性创意扩写 |
| specs | 规格参数 | 尺寸、颜色、功能清单、对比表格 | 1:1 / 9:16 | 严格复用所有规格数据 |
| usage | 使用场景 | 演示产品如何使用、步骤图解 | 3:4 / 1:1 | 产品外观；使用步骤可创意编排 |
| comparison | 对比图 | 前后对比、竞品对比、升级对比 | 1:1 / 16:9 | 对比事实、数据、产品版本 |
| social | 社交卡片 | 小红书、朋友圈、Instagram 分享图 | 1:1 / 3:4 | 产品主体；风格/文案可创意 |

---

## Prompt 模板结构

每张海报 prompt 统一输出以下字段，供 Agent 构造最终 `generate-image` 调用：

| 字段 | 说明 |
|------|------|
| `sceneType` | 类型标识（上表之一） |
| `purpose` | 该海报在电商素材体系中的角色（1 句话） |
| `prompt` | 英文生图提示词（符合 `generate-image` 工具要求，2-5 句） |
| `styleConstraints` | 风格约束，映射到 `style-gallery.md` 中的 preset 名称 + 关键词 |
| `textLayoutRules` | 文字排版规则：位置、层级、字体建议 |
| `bilingualGuidelines` | 中英双语说明：中文面向运营的标题/说明 + 英文注入 prompt 的技术关键词 |
| `aspectRatio` | 建议比例（`auto`, `1:1`, `3:4`, `4:3`, `16:9`, `9:16` 等） |
| `mustPreserve` | 必须严格复用的产品事实列表（外观、颜色、材质、规格、卖点） |
| `creativeLicense` | 允许创意扩写的范围（背景、氛围、搭配、构图角度） |

---

## 风格约束映射

所有 style 引用必须指向仓库预设风格（style presets）。电商场景常用映射如下：

| 电商场景 | 推荐 style-gallery preset | 理由 |
|----------|--------------------------|------|
| 高端品牌主视觉 | `corporate-memphis` | 扁平矢量 + 鲜明色块，适合 SaaS/科技产品 |
| 生活方式/情感连接 | `watercolor-storybook` | 水彩柔和，适合生活方式、治愈系产品 |
| 科技/数码产品 | `cyberpunk-neon` | 深色背景 + 霓虹发光，突出科技感 |
| 食品/可爱产品 | `claymation` | 3D 黏土质感，圆润可爱 |
| 工具/清单/开箱 | `knolling` | 正俯拍陈列，整齐排列 |
| 教育/教程类产品 | `chalkboard` | 黑板粉笔风，知识感 |
| 复古/文化/手工 | `aged-academia` | 泛黄纸张 + 铜版画线条 |
| 极简/高端/留白 | `ui-wireframe` 或五维体系 `minimal` | 灰度线框或极简构图 |
| 趣味/年轻/Pop | `bold-graphic` | 粗线漫画 + 半调网点 |
| 默认（无明确信号） | 不推荐风格，走五维体系默认组合 | Type + Palette + Rendering + Text + Mood，不额外指定风格 |

**禁止**：定义新的 style preset 或发明不在仓库预设风格列表中的风格名称。

---

## 排版与字体规则

### 文字层级建议

| 层级 | 位置 | 内容 | 字体建议 |
|------|------|------|----------|
| 主标题 | 画面上方 1/3 或中央 | 产品名 / 活动主题 | 粗体无衬线（Bold Sans-serif） |
| 副标题 | 主标题下方 | 卖点 / 促销信息 | 中等粗细无衬线 |
| 标签/价格 | 画面角落或底部 | 价格、规格、限时标签 | 等宽或紧凑无衬线 |
| 品牌标识 | 画面角落 | Logo 区域 | 保持品牌字体一致性 |

### 排版原则

- 主标题字数控制在 8 个汉字以内，英文不超过 5 个单词
- 促销信息（如 "50% OFF"）使用高对比色块突出
- 价格数字使用等宽字体确保对齐感
- 避免文字覆盖产品主体；文字区域与产品区域保持安全边距
- 多语言场景：中文标题 + 英文副标题，或反之，保持视觉平衡

**注意**：字体建议仅作为 prompt 中的描述词使用（如 "bold sans-serif typography"）。实际可渲染字体能力取决于 `generate-image` 工具的 `includeTextInImage` 和底层模型支持。不引入新的 font system。

---

## 双语规范

- **中文部分**：面向运营人员的标题、说明、场景描述。用于 Agent 内部理解和用户沟通。
- **英文部分**：注入 `generate-image` prompt 的技术关键词。Prompt 字段必须使用英文（模型性能更优）。

**示例**：

```text
中文：主视觉海报 — 夏季清凉促销，突出产品冰凉质感
英文 prompt：Summer cool promotion hero banner, refreshing icy texture,
product centered with dramatic cold lighting, bold sans-serif headline,
vivid blue and white color scheme, high contrast commercial photography style
```

---

## 数量灵活说明

- **不预设固定数量**：Agent 根据用户意图决定生成几张海报。用户说 "帮我做几张产品图" 时，先 ask_user 确认数量和类型偏好。
- **首图模式**：用户未明确数量时，可先生成 1 张 hero 主视觉作为风格确认，用户满意后再批量生成其余类型。
- **批量并发**：当需要一次性生成多张同风格电商海报时，可使用 `image-generate-ai-batch` 工具并发执行；串行生成仍为默认路径（兼容现有行为）。
- **组合建议**：典型电商素材套装 = hero(1) + lifestyle(1-2) + detail(1) + specs(1) + social(1-2)，Agent 根据产品复杂度和用户预算灵活调整。

---

## 边界说明

- **不适用于**：infographics / 数据可视化、slideshow / 多页、video、template editing。这些场景由其他 skill 处理。
- **与 Product 类型的关系**：电商海报是 `Product` 图片类型下的一个子场景，不新增独立 Image Type。
- **与 slideshow 的边界**：电商海报是单张静态图；若用户需要多张轮播或带翻页动画，转交 slideshow skill。
