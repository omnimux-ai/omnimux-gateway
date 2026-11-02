# Inspiration Candidate Display

在 `editor-match-slideshow-inspirations` 返回候选卡片后，把 `recognizedParams.inspirationCandidates[*].display` 视为仅供确认骨架的展示层字段。

## 字段语义

- `title`: 候选标题，仅用于帮助用户辨认候选风格
- `summary`: 候选的高层摘要，用于判断是否值得采用该骨架
- `previewSlides`: 预览页摘要，只用于确认叙事节奏和结构，不得直接当作最终成稿
- `keyPoints`: 候选亮点或推荐理由，用于解释系统为何推荐该候选
- `cover` / `coverImage` / `thumbnail`: 缩略视觉锚点，仅用于卡片展示

## 使用规则

1. 先让用户基于 `display.*` 判断“这是不是合适的骨架”。
2. 用户确认后，只把抽象骨架信息写回会话：`selectedInspirationId`、`replicationMode`、`replicationSourceId`、`replicationAnchor`、`matchConfidence`、`selectionReason`。
3. 需要模板正文、完整页结构或持久化模板内容时，交给 `editor-start-slideshow-generate` 基于 `selectedInspirationId` 在服务端补齐。

## 禁止事项

- 不要把 `previewSlides`、`summary`、`keyPoints` 直接拼接进最终文案
- 不要把卡片展示字段当成模板真相源覆盖服务端模板详情
