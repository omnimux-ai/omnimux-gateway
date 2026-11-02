# Video Storyboard Output Contract

输出保持统一 JSON 结构：

```json
{
  "title": "视频标题",
  "hook": "开场吸引语（前3秒）",
  "scenes": [
    {
      "id": "1",
      "duration": 8,
      "description": "画面描述",
      "narration": "旁白文案",
      "transition": "转场效果建议",
      "interClipBoundary": "scene_cut | continuous",
      "audioRouting": {
        "onScreenDialogue": null,
        "onScreenSinging": null,
        "narrationCue": null,
        "bgmSource": "embedded | separate | none",
        "bgmCue": null,
        "soundEffects": null
      }
    }
  ],
  "cta": "结尾行动号召（可选）",
  "caption": "视频简介/文案"
}
```

## 强制规则

- `hook` 负责前 3 秒抓手，不能留空
- `scenes[*].description` 必须满足正文里的四要素和最少句数要求
- `audioRouting.onScreenDialogue` 或 `onScreenSinging` 有值时，`narrationCue` 必须为 `null`
- `bgmSource=separate` 时，只能指向版权免费音乐搜索路径，不能改写成 AI 背景音乐
