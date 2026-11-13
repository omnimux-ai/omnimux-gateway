# 视觉风格库

> 位置：`server/skills/image/image-creation/references/style-gallery.md`
> 用途：预定义视觉风格集合，Agent 按内容信号自动推荐或供用户选择
> 来源：适配自 baoyu-infographic 风格体系 + baoyu-cover-image 风格预设

---

## 使用方式

1. Agent 根据用户需求的内容信号（见「内容信号 → 风格映射」）推荐风格
2. 将风格的 Prompt 关键词融入五维体系的 Rendering 和 Palette 维度
3. 用户明确指定风格时直接使用，不推荐时走五维体系默认组合

---

## 风格定义

### 1. craft-handmade（手工纸艺）

- **视觉特征**：剪纸质感、手撕边缘、纸面纹理、温暖阴影
- **适用场景**：教育内容、生活类、手工DIY、亲子
- **推荐维度**：Palette=warm/earth, Rendering=hand-drawn, Mood=subtle
- **Prompt 关键词**：`paper craft illustration, cut paper collage, handmade paper texture, torn edges, warm shadow, layered paper cutout style`

### 2. corporate-memphis（企业孟菲斯）

- **视觉特征**：扁平矢量人物、鲜明色块、几何形状、无阴影
- **适用场景**：SaaS 产品、商业报告、科技公司、产品功能介绍
- **推荐维度**：Palette=vivid, Rendering=flat-vector, Mood=balanced
- **Prompt 关键词**：`corporate memphis style, flat vector characters, bold color blocks, geometric shapes, no shadows, tech startup illustration`

### 3. technical-schematic（技术蓝图）

- **视觉特征**：蓝底白线、网格背景、工程图样式、标注线
- **适用场景**：架构图、系统设计、技术文档、工程类
- **推荐维度**：Palette=cool, Rendering=flat-vector, Mood=balanced
- **Prompt 关键词**：`blueprint technical drawing, white lines on dark blue, grid background, engineering schematic, annotation lines, technical diagram`

### 4. chalkboard（黑板粉笔）

- **视觉特征**：黑板底色、彩色粉笔线条、手写体、教室氛围
- **适用场景**：教育、教程、课堂笔记、知识科普
- **推荐维度**：Palette=dark, Rendering=chalk, Mood=balanced
- **Prompt 关键词**：`colorful chalk drawing on blackboard, hand-lettered, dusty chalk texture, classroom aesthetic, educational chalkboard illustration`

### 5. watercolor-storybook（水彩绘本）

- **视觉特征**：水彩渗透效果、柔和边缘、梦幻质感、童话感
- **适用场景**：儿童内容、生活方式、治愈系、情感类
- **推荐维度**：Palette=pastel, Rendering=painterly, Mood=subtle
- **Prompt 关键词**：`soft watercolor illustration, dreamy storybook art, gentle color blending, fairy tale aesthetic, whimsical watercolor painting`

### 6. cyberpunk-neon（赛博霓虹）

- **视觉特征**：深色背景、霓虹发光、科技线条、未来感
- **适用场景**：科技前沿、AI/VR、游戏、未来主题
- **推荐维度**：Palette=dark, Rendering=digital, Mood=bold
- **Prompt 关键词**：`cyberpunk neon glow, dark background with glowing edges, futuristic holographic, sci-fi neon lights, electric blue and magenta`

### 7. bold-graphic（粗线漫画）

- **视觉特征**：粗描边、半调网点、漫画分格、强对比
- **适用场景**：趣味内容、创意营销、年轻受众、Pop Art
- **推荐维度**：Palette=vivid, Rendering=flat-vector, Mood=bold
- **Prompt 关键词**：`bold comic style, thick outlines, halftone dots, pop art graphic, high contrast, comic book illustration`

### 8. aged-academia（复古学术）

- **视觉特征**：泛黄纸张、铜版画线条、手绘标本、旧式标注
- **适用场景**：历史、科学、博物馆、文化类
- **推荐维度**：Palette=retro/earth, Rendering=hand-drawn, Mood=subtle
- **Prompt 关键词**：`vintage academic illustration, aged parchment paper, copperplate engraving style, botanical drawing, antique scientific diagram`

### 9. pixel-art（像素艺术）

- **视觉特征**：方块像素、8-bit 色彩、复古游戏、怀旧感
- **适用场景**：游戏、开发者内容、怀旧主题、极客文化
- **推荐维度**：Palette=vivid, Rendering=pixel, Mood=balanced
- **Prompt 关键词**：`pixel art, 8-bit retro game style, chunky pixels, limited color palette, nostalgic video game aesthetic`

### 10. ikea-manual（宜家手册）

- **视觉特征**：极简线条、无文字说明、步骤图解、蓝红双色
- **适用场景**：操作指南、产品说明、流程图解、How-to
- **推荐维度**：Palette=mono, Rendering=flat-vector, Mood=subtle
- **Prompt 关键词**：`ikea instruction manual style, minimal line art, simple step illustration, blue and red accent, wordless diagram`

### 11. claymation（黏土动画）

- **视觉特征**：3D 黏土质感、圆润造型、定格动画、可爱立体
- **适用场景**：儿童产品、食品、可爱角色、品牌吉祥物
- **推荐维度**：Palette=warm/vivid, Rendering=digital, Mood=balanced
- **Prompt 关键词**：`claymation 3D style, clay figure, stop-motion aesthetic, rounded soft shapes, playful sculpted characters`

### 12. knolling（俯拍陈列）

- **视觉特征**：正俯视角、物品整齐排列、统一间距、背景纯色
- **适用场景**：产品组合、工具集、清单类、开箱展示
- **推荐维度**：Palette=auto, Rendering=digital, Mood=subtle
- **Prompt 关键词**：`knolling flat lay, overhead view, neatly organized objects, uniform spacing, clean background, curated arrangement`

### 13. origami（折纸几何）

- **视觉特征**：折纸造型、几何棱面、可见折痕、投射阴影、低多边形质感
- **适用场景**：几何概念、变化/转型主题、日式美学、抽象表达
- **推荐维度**：Palette=vivid, Rendering=digital, Mood=balanced
- **Prompt 关键词**：`origami folded paper style, geometric faceted forms, visible fold lines, cast shadows showing depth, low-poly paper aesthetic, angular shapes on clean background`

### 14. kawaii（卡哇伊）

- **视觉特征**：大眼角色、柔和粉彩色、圆润气泡形状、腮红、星星闪光、Q版比例
- **适用场景**：可爱教程、儿童教育、生活方式、角色驱动解说
- **推荐维度**：Palette=pastel, Rendering=digital, Mood=subtle
- **Prompt 关键词**：`kawaii Japanese cute style, big sparkly eyes, soft pastel pink mint lavender, rounded bubbly shapes, blushing cheeks, sparkles and stars, chibi characters`

### 15. subway-map（地铁图）

- **视觉特征**：彩色路线线条、45°/90°角、站点圆点标记、换乘符号、简化地理
- **适用场景**：旅程图、流程网络、路线解说、知识图谱
- **推荐维度**：Palette=vivid, Rendering=flat-vector, Mood=balanced
- **Prompt 关键词**：`subway transit map style, colored route lines at 45° and 90° angles, station circle markers, interchange symbols, simplified geography, clean sans-serif labels`

### 16. lego-brick（乐高积木）

- **视觉特征**：可见积木凸点、模块化构建、经典原色（红蓝黄绿）、塑料光泽、拼装说明书风
- **适用场景**：构建概念、模块化系统、趣味教育、儿童内容
- **推荐维度**：Palette=vivid, Rendering=digital, Mood=balanced
- **Prompt 关键词**：`lego brick construction style, visible brick studs, modular stackable elements, classic primary colors (red blue yellow green), plastic sheen, building instruction aesthetic`

### 17. ui-wireframe（线框原型）

- **视觉特征**：灰度界面、占位框、极简线条、技术规格感、网格对齐
- **适用场景**：技术文档、产品原型、功能规格、开发者内容
- **推荐维度**：Palette=mono, Rendering=flat-vector, Mood=subtle
- **Prompt 关键词**：`grayscale UI wireframe style, clean interface mockup, placeholder boxes, minimal line art, technical specification layout, grid-aligned components`

---

## 内容信号 → 风格映射

Agent 根据用户输入的关键词/语境自动推荐风格：

| 内容信号 | 推荐风格 |
|---------|---------|
| 教程、学习、教育、入门 | chalkboard 或 craft-handmade |
| 技术、架构、系统、API | technical-schematic |
| 产品、SaaS、商业、报告 | corporate-memphis |
| AI、科技前沿、VR、未来 | cyberpunk-neon |
| 儿童、亲子、治愈、绘本 | watercolor-storybook 或 claymation |
| 历史、文化、科学、博物馆 | aged-academia |
| 游戏、开发者、极客 | pixel-art |
| 操作指南、步骤、How-to | ikea-manual |
| 产品清单、开箱、工具集 | knolling |
| 趣味、创意、年轻、Pop | bold-graphic |
| 几何、变化、日式、折纸 | origami |
| 可爱、萌系、儿童、角色 | kawaii |
| 路线、网络、流程、连接 | subway-map |
| 构建、模块、拼装、积木 | lego-brick |
| 原型、线框、规格、开发 | ui-wireframe |
| 默认（无明确信号） | 不推荐风格，走五维体系默认组合 |

---

## 风格与五维体系的关系

风格是五维体系的**预设快捷方式**。选择风格等于一次性设定 Palette + Rendering + Mood 三个维度。

Type 和 Text 维度仍由图片类型决定，不受风格影响。

如果用户同时指定了风格和某个维度（如「用 chalkboard 风格但要 vivid 色彩」），以用户指定的维度值覆盖风格预设。
