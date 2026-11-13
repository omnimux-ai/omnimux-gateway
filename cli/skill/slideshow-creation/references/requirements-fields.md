# Requirements Collection — Field Definitions

Complete `ask_user` call example with all field groups.

## Full Example

```json
{
  "missingFields": [
    {
      "fieldName": "prompt",
      "fieldLabel": "创作主题",
      "fieldType": "textarea",
      "required": true,
      "fieldCategory": "required"
    },
    {
      "fieldName": "targetPlatform",
      "fieldLabel": "发布平台",
      "fieldType": "select",
      "required": true,
      "fieldCategory": "required",
      "options": [
        { "value": "rednote", "label": "小红书" },
        { "value": "tiktok", "label": "TikTok" },
        { "value": "douyin", "label": "抖音" },
        { "value": "instagram", "label": "Instagram" }
      ]
    },
    {
      "fieldName": "slideType",
      "fieldLabel": "内容侧重点",
      "fieldType": "select",
      "required": true,
      "fieldCategory": "required",
      "options": [
        { "value": "educational", "label": "干货知识" },
        { "value": "personal", "label": "个人分享" },
        { "value": "marketing", "label": "营销推广" }
      ]
    },
    {
      "fieldName": "imageSourceMode",
      "fieldLabel": "配图方式",
      "fieldType": "select",
      "required": true,
      "fieldCategory": "required",
      "defaultValue": "auto",
      "options": [
        { "value": "auto", "label": "自动" },
        { "value": "smart", "label": "智能匹配图库" },
        { "value": "ai", "label": "AI生成" }
      ]
    },
    {
      "fieldName": "sellingPoints",
      "fieldLabel": "产品卖点",
      "fieldType": "textarea",
      "required": false,
      "fieldCategory": "custom",
      "placeholder": "例如：续航长、轻便、性价比高"
    },
    {
      "fieldName": "promotion",
      "fieldLabel": "特价/促销",
      "fieldType": "text",
      "required": false,
      "fieldCategory": "custom",
      "placeholder": "例如：限时 8 折、买一送一"
    },
    {
      "fieldName": "targetAudience",
      "fieldLabel": "目标受众",
      "fieldType": "textarea",
      "required": false,
      "fieldCategory": "custom",
      "placeholder": "例如：学生党、通勤上班族"
    }
  ],
  "message": "请补充核心创作信息；其中配图方式为必填项。"
}
```

## Field Categories

- `required` — must be completed before generation
- `suggested` — recommended, default expanded in UI
- `custom` — optional, default collapsed in UI

## Canonicalization Notes

- `templateType` / `contentType` / `slide_type` are canonicalized to `slideType` by `ask_user`.
- For `domain=slideshow`, unknown `required` fields are downgraded to `custom` optional fields to prevent repeat suspend loops.
- `imageSourceMode` 默认值为 `auto`（自动）；前端可能基于用户上次选择记忆覆盖默认值（以当前 options 白名单为准）。
- `inspirationSource` 是“复刻骨架”的主要载体；当 Agent 已确定骨架时，建议同时传递：
  - `replicationMode`
  - `replicationSourceId`
  - `replicationAnchor`
  - `matchConfidence`
  - `selectionReason`
- 如果只传了 `replicationSourceId` / `selectionReason` / `matchConfidence` 等骨架字段，却没有可确定 `replicationMode` 的上下文，`editor-start-slideshow-generate` 会直接拒绝。
- `replicationMode=external_imported` 时必须同时携带 `inspirationSource` 和 `replicationSourceId`。
- 对于 `editor-match-slideshow-inspirations` 的挂起确认卡片，`recognizedParams.inspirationCandidates[*]` 还应提供：
  - `display.previewMode`：`slideshow | video | image | text`，明确前端该如何渲染预览
  - `display.previewImageUrl`：模板封面首图，可为空
  - `display.previewSlides[]`：图文模板预览页数组；应优先透传灵感库真实内容，而不是只给摘要
  - `display.previewVideoUrl`：视频模板预览地址；有视频时不要退化成纯文字卡片
  - `display.summary`：模板内容摘要，前端首屏必须直接展示
  - `display.keyPoints[]`：模板关键句/结构亮点，避免无图时退化为空白卡片
- 若用户显式选择“不使用灵感模板，直接生成”，应传 `replicationMode=user_skipped`，不要只靠缺失 `inspirationSource` 表达。
- `replicationMode=user_skipped` 时，不得再携带 `inspirationSource`、`selectedInspirationId` 或 `replicationSourceId`。

## Chat Config Mapping

Frontend `imageTextConfig.imageSource` → `imageSourceMode`:
- `auto` → `auto`
- `ai-generate` → `ai`
- `ai-recommend` → `smart`
