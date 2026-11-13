# TikTok Photo Mode Overlay

用途：在 `slideshow-creation` 已激活后，为 TikTok Photo Mode 图文补充平台差异规则。

## 何时读取

- `targetPlatform/platform/platformHint` 明确是 `tiktok`
- 用户明确要求 TikTok Photo Mode / carousel / swipe-left 风格

## 深度参考（按需）

- `references/platforms/tiktok/skill-context.json`
- `references/platforms/tiktok/TikTok视觉钩子策略.md`
- `references/platforms/tiktok/TikTok轮播叙事节奏.md`
- `references/platforms/tiktok/TikTok文案与SEO策略.md`

## 边界

- 这是平台 overlay，不是主 workflow
- 不能替代 `slideshow-creation`
- 不负责 requirements collection、research、persist 等主链路决策

## 平台差异规则

### 文案与信息密度

- 每页只讲一个最小信息块，优先短句、大字、强节奏
- 第一页必须做到 `Stop the Scroll`
- 文案要更像“滑动中的视觉信息”，不是完整长笔记

### 页面节奏

- 首页抛钩子，中间页递进拆解，尾页给结论/CTA
- 强调跨页悬念和 swipe 继续阅读动机
- `visualHint` 应明确 overlay text、箭头、对比、跨页拼接等提示

### 视觉风格

- 更强调强反差、封面钩子、单页冲击
- 避免小红书式过重的“笔记感排版”
- 更适合 checklist、before/after、step-by-step、POV 结构

## 输出检查

- 首页是否在 0.5 秒内说明“为什么要继续滑”
- 是否每页只有一个最小可读结论
- 是否避免了“小红书笔记式长段表达”
