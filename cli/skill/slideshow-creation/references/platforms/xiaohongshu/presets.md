# 小红书 Preset 快捷方式

> 位置：`references/platforms/xiaohongshu/presets.md`
> 用途：为 Agent 提供「场景 → 风格+布局」的一键映射，减少逐步推导的决策成本
> 用法：当用户描述场景或 `analysis-framework.md` 完成分析后，Agent 直接按 preset 锁定 style + layout

---

## 知识与学习类

| Preset | Style | Layout | 适合场景 |
|--------|-------|--------|---------|
| `knowledge-card` | notion | dense | 干货知识卡、概念科普、思维模型 |
| `checklist` | notion | list | 清单、排行榜、必备清单、核对表 |
| `concept-map` | notion | mindmap | 概念图谱、知识脉络、关系梳理 |
| `swot` | notion | quadrant | SWOT 分析、四象限分类、优先级 |
| `tutorial` | chalkboard | flow | 教程步骤、操作流程、怎么做 |
| `classroom` | chalkboard | balanced | 课堂笔记、知识讲解、科普文 |
| `study-guide` | study-notes | dense | 学习笔记、考试重点、读书摘录 |

---

## 生活方式与分享类

| Preset | Style | Layout | 适合场景 |
|--------|-------|--------|---------|
| `cute-share` | cute | balanced | 少女风日常分享、种草笔记 |
| `girly` | cute | sparse | 甜美封面、氛围感、美妆开箱 |
| `cozy-story` | warm | balanced | 生活故事、情感分享、亲子记录 |
| `product-review` | fresh | comparison | 产品对比、横评测评、选购建议 |
| `nature-flow` | fresh | flow | 健康流程、有机生活、打卡记录 |

---

## 观点与冲击类

| Preset | Style | Layout | 适合场景 |
|--------|-------|--------|---------|
| `warning` | bold | list | 避坑指南、重要提醒、血泪教训 |
| `versus` | bold | comparison | 正反对比、强烈对照、选边站 |
| `clean-quote` | minimal | sparse | 金句封面、极简卡片、名言分享 |
| `pro-summary` | minimal | balanced | 专业总结、商务内容、品牌种草 |

---

## 潮流与娱乐类

| Preset | Style | Layout | 适合场景 |
|--------|-------|--------|---------|
| `retro-ranking` | retro | list | 复古排行、经典盘点、年代感 |
| `throwback` | retro | balanced | 怀旧分享、老物件、回忆杀 |
| `pop-facts` | pop | list | 趣味冷知识、好玩的事、惊喜发现 |
| `hype` | pop | sparse | 炸裂封面、惊叹分享、新品发布 |

---

## 海报与评论类

| Preset | Style | Layout | 适合场景 |
|--------|-------|--------|---------|
| `poster` | screen-print | sparse | 海报风封面、影评书评、大片感 |
| `editorial` | screen-print | balanced | 观点文章、文化评论、深度分析 |
| `cinematic` | screen-print | comparison | 电影对比、戏剧张力、视觉叙事 |

---

## 使用说明

1. **Agent 选 preset**：完成 `analysis-framework.md` 分析后，从上表中找到最匹配场景的 preset
2. **覆盖单项**：preset 只是起点，可单独覆盖 style 或 layout（如 `cute-share` 但改 layout 为 `list`）
3. **用户指定优先**：若用户明确说"用知识卡风格"等，直接匹配对应 preset，无需再推导

> 风格详情见：`visual-styles.md` | 布局详情见：`layout-and-strategy.md`
