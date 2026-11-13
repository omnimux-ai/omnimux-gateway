# Xiaohongshu Platform Overlay

用途：在 `slideshow-creation` 已激活后，为小红书图文补充平台差异规则。

## 何时读取

- `targetPlatform/platform/platformHint` 明确是 `xiaohongshu` 或 `rednote`
- 用户明确要求"小红书风格/小红书笔记语气/小红书封面标题"

## 深度参考（按需读取）

| 文件 | 何时读 |
|------|--------|
| `references/platforms/xiaohongshu/skill-context.json` | 任何 XHS 创作，必读，核心技巧 |
| `references/platforms/xiaohongshu/analysis-framework.md` | Copy Generation 前，分析内容类型→Strategy |
| `references/platforms/xiaohongshu/presets.md` | 选定 preset 快捷方式 |
| `references/platforms/xiaohongshu/prompt-assembly.md` | 生成 visualHint 时，组装模板 |
| `references/platforms/xiaohongshu/layout-and-strategy.md` | 了解布局类型和 Strategy A/B/C 结构 |
| `references/platforms/xiaohongshu/visual-styles.md` | 了解 11 种视觉风格定义 |
| `references/platforms/xiaohongshu/小红书标题&封面文案策略.md` | 标题封面需要高质量时 |
| `references/platforms/xiaohongshu/小红书笔记的成篇与撰写.md` | 正文质量需要提升时 |
| `references/platforms/xiaohongshu/output-format.md` | 历史合同，仅参考 |

## 边界

- 这是平台 overlay，不是主 workflow
- 不能替代 `slideshow-creation`
- 不负责 ask_user / 研究 / 配图 / persist 的主顺序决策

---

## Copy Generation 前的 Style 预锁定（关键）

**在开始生成文案（Step 5）之前，必须先完成以下决策：**

### 步骤 1：内容分析

按 `analysis-framework.md` 的快速分析模板，基于 `prompt + sellingPoints + targetAudience` 完成内部分析，得出：
- 内容类型
- 推荐 Strategy（A/B/C）
- 推荐 preset

### 步骤 2：Auto-Selection（内容信号 → preset）

若用户未明确指定风格，按以下映射自动选择：

| 内容信号 | 推荐 preset | Strategy |
|---------|-----------|---------|
| 护肤/美妆/穿搭/少女/日常种草 | `cute-share` / `girly` | A |
| 健康/有机/清爽/自然/运动 | `product-review` / `nature-flow` | B/C |
| 生活故事/情感/亲子/美食温馨 | `cozy-story` | A |
| 避坑/警告/重要提醒/血泪 | `warning` / `versus` | B |
| 干货/知识/思维模型/效率 | `knowledge-card` / `checklist` | B |
| 教程/步骤/操作流程 | `tutorial` / `classroom` | B |
| 学习笔记/考试/读书 | `study-guide` | B |
| 极简/高端/品牌/专业总结 | `clean-quote` / `pro-summary` | B/C |
| 复古/怀旧/老物件 | `throwback` / `retro-ranking` | A/B |
| 潮流/新品/趣味/年轻 | `hype` / `pop-facts` | C/B |
| 影评/书评/观点文章/海报感 | `poster` / `editorial` | C |
| 电影/戏剧对比/文化评论 | `cinematic` | C |

### 步骤 3：Strategy → 文案结构

选定 Strategy 后，按以下结构组织文案：

**Strategy A（故事驱动）** — 适合种草/情感/个人经历
```
P1 封面：Hook 痛点共鸣（"入冬后脸不干了🥹"）← sparse
P2 痛点：之前的困扰和失败尝试 ← balanced
P3 发现：发现产品/方法的契机 ← balanced
P4 体验：使用感受、效果细节 ← balanced/dense
P5 结论：总结 + CTA ← sparse
```

**Strategy B（信息密集）** — 适合干货/测评/教程/清单
```
P1 封面：价值承诺（"5款平价精华横评"）← sparse
P2 总览：核心结论或全览表 ← list/dense
P3 详解：逐项详细对比 ← dense/comparison
P4 推荐：排名或最终建议 ← list
P5 CTA：互动引导 + 收藏提醒 ← sparse
```

**Strategy C（视觉优先）** — 适合高颜值产品/旅行/穿搭
```
P1 封面：主视觉大图 + 一句话标题 ← sparse
P2 细节：产品/场景细节特写 ← sparse/balanced
P3 氛围：生活场景/使用场景 ← sparse
P4 CTA：简短总结 + 互动引导 ← sparse
```

---

## 文案调性规则

- 优先"真实经验 + 利他分享"，避免太像广告脚本
- 允许更强的第一人称和生活化表达，但不要假装真人经历
- 标题应突出"有用/避坑/清单/对比/推荐理由"
- 禁止使用学术名词、比喻概念、双引号等 AI 特征词汇
- 标题不超过 17 字，封面文案更短

## 页面节奏规则

- 首页必须快速说明"你为什么要看这组图文"
- 中间页保持一页一个重点，避免长段小作文
- 末页用总结、选择建议、购买/行动提醒做收束

## 标题与封面

- 标题要具体，不要抽象口号
- 封面文案优先"痛点 + 价值结果"，而不是空泛情绪词
- 对比、避坑、清单、入门推荐类标题通常更稳

---

## visualHint 质量标准

生成每页 `visualHint` 时，**必须体现**：
1. **风格关键词**（从已选 style 的关键词组中取 2-3 个词）
2. **布局类型**（sparse/balanced/dense/list/comparison/flow/mindmap/quadrant）
3. **页面核心内容**（该页主视觉焦点或信息结构）

**不合格示例**：`温暖的图文排版`（无风格词、无布局类型）
**合格示例**：`暖黄奶油色背景，warm 风格，balanced 布局，上方产品特写图，下方标题「入冬护肤3步」加粗，温馨生活感`

详细组装模板见：`prompt-assembly.md`

---

## 生成前 Checklist（3 项）

- [ ] Style 已确定（已选 preset 或明确 style 名）
- [ ] Strategy 已选（A/B/C，影响全组文案结构）
- [ ] 每页 visualHint 体现 layout 类型（非模糊描述）

---

## 输出检查

- 是否像"可收藏/可转发/可执行"的笔记，而不是品牌硬广
- 是否有明确适用人群、预算/场景/优缺点等实用信息
- 是否避免了"平台错位"表达，例如 TikTok 式极短碎片化标题
