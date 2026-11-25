# 火山引擎豆包语音合成 3.0 官方音色库检索手册（Agent 索引规范）

本文档为 Agent、业务系统及开发者提供火山引擎 **豆包语音合成模型 3.0（Doubao-Seed-TTS-3.0）** 官方预置音色库的速查表与网关调用指引。
完整数据源：`docs/assets/volcengine-voices.json`（与 `cli/src/data/voices.json` 同源，收录 68 款 3.0 官方音色）。

---

## 一、调用技术规范（免克隆直用）

* **网关入口**：`POST /v1/audio/speech`（公开模型 ID：`seed-tts-3.0`）
* **上游官方接口**：`POST https://voice.cn-beijing.volces.com/api/v3/text-to-speech/{voice_id}/streaming`（`model_id: "doubao-seed-tts-3.0"`）
* **计费口径**：按输入字符计费（汉字/日文汉字计 2 字符，字母/假名/标点/空格/SSML 标签计 1 字符；单次上限 10,000 字符）
* **能力支持**：全量 68 款 3.0 音色均支持指令遵循（`instructions`）、跨语种合成（119 项语种/方言，`language_code`）及 Inline 情感标签。

---

## 二、代表音色速查表（按场景分类）

### 1. 通用与品牌播报
| 音色名称 | 官方 `voice_type` 代号 | 语种 | 场景 |
| :--- | :--- | :--- | :--- |
| **Vivi 3.0** | `multi_female_vv_xi_v3_uranus_bigtts` | 中文 | 陪聊 / 通用默认 |
| **小天 3.0** | `multi_male_taocheng_v3_uranus_bigtts` | 中文 | 通用场景 |
| **云舟 3.0** | `multi_male_m191_v3_uranus_bigtts` | 中文 | 通用场景 |
| **知性女声 3.0** | `multi_female_zhixingnv_v3_uranus_bigtts` | 中文 | 通用场景 |
| **温柔小雅 3.0** | `multi_female_wenrouxiaoya_v3_uranus_bigtts` | 中文 | 通用场景 |
| **团子 3.0** | `multi_female_tuanzi_v3_uranus_bigtts` | 中文 | 通用场景 |

### 2. 视频配音、带货营销与有声阅读
| 音色名称 | 官方 `voice_type` 代号 | 语种 | 特色标签 |
| :--- | :--- | :--- | :--- |
| **活力营销女 3.0** | `multi_female_huoliyingxiao_nv_v3_uranus_bigtts` | 中文 | 客服 / 营销 |
| **悬疑解说 3.0** | `multi_male_xuanyijieshuo_v3_uranus_bigtts` | 中文 | 抖音同款、豆包同款、剪映同款 |
| **深夜播客 3.0** | `multi_male_shenyeboke_v3_uranus_bigtts` | 中文 | 有声阅读 |
| **TVB女声 3.0** | `multi_female_tvbnv_v3_uranus_bigtts` | 中文 | 抖音同款、豆包同款 |
| **佩奇猪 3.0** | `multi_female_peiqi_v3_uranus_bigtts` | 中文 | 抖音同款、豆包同款、剪映同款 |
| **安逸川妹 3.0** | `multi_female_sichuandialectfemalevoicedb_3_v3_uranus_bigtts` | 四川话 | 陪聊、趣味口音 |

### 3. 外语音色（美式 / 英式英语）
| 音色名称 | 官方 `voice_type` 代号 | 语种 | 场景 |
| :--- | :--- | :--- | :--- |
| **Jamie 3.0** | `multi_male_jamie_v3_uranus_bigtts` | 英语（美式口音） | 通用场景、教学场景 |
| **Myra 3.0** | `multi_female_myra_v3_uranus_bigtts` | 英语（美式口音） | 教学场景 |
| **Jenny 3.0** | `multi_female_jenny_v3_uranus_bigtts` | 英语（美式口音） | 客服场景 |
| **Stokie 3.0** | `multi_female_stokie_v3_uranus_bigtts` | 英语（英式口音） | 陪聊、英式英语 |

---

## 三、CLI 检索与网关 API 消费契约

### 1. CLI 检索全部 68 款音色
```bash
omnimux voices --model seed-tts-3.0
omnimux voices --query 深夜播客
omnimux voices --scene 电商带货
```

### 2. 网关 `/v1/audio/speech` 请求示例
```json
{
  "model": "seed-tts-3.0",
  "input": "[happy]大家好，欢迎收听今天的节目。",
  "voice": "multi_male_shenyeboke_v3_uranus_bigtts",
  "language_code": "zh",
  "instructions": "语气温柔自然，略带播客磁性",
  "response_format": "mp3",
  "speed": 1.0
}
```
- 需要字词级字幕时间戳时，传 `"enable_subtitle": true` 并将 `"response_format"` 设为 `"json"`（或携带 `Accept: application/json`）。
- 需要原始 JSONL 流式事件时，携带 `Accept: application/x-ndjson`。
