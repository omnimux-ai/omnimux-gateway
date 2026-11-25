# 火山引擎音色库解耦架构与运维规范（Control-Data Plane Separation）

## 1. 架构背景与问题定性

在接入火山引擎声音复刻（ICL）与大模型语音合成（TTS）能力时，传统思维容易陷入以下误区：
* **误区假设**：业务网关需要搭建一套音色文件托管服务，或者每次调用时去火山拉取音色音频文件，然后再把该音频文件作为“参考音频（Reference/Prompt Audio）”透传给音频生成接口。
* **事实与弊端**：
  1. **接口不支持常态文件托管**：火山音色设计/训练接口返回的试听音频（`demo_audio`）仅有 **1 小时临时签名有效期**，不可作为持久化音频源使用；
  2. **网络与带宽浪费巨大**：每次合成都跨网络拉取并转发几兆的音频文件，直接导致首包延迟（TTFT）增加数秒；
  3. **模型算力浪费**：大模型每次都要重新计算和提取参考音频的特征向量，无法利用云端预加载优化；
  4. **网关臃肿**：迫使核心网关引入复杂的对象存储（OSS/S3）、文件缓存及重型任务状态机。

## 2. 核心架构裁决：控制面与数据面彻底分离

经评审裁决，系统采用**“控制面与数据面分离（Control-Data Plane Separation）”**模式：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                       控制面（Control Plane · 运维范畴）                 │
│                                                                         │
│   运营/录音音频/提示词 ──► [运维CLI工具 scripts/ops/volcengine-voice/]  │
│                                           │                             │
│                                           ▼                             │
│                       火山音色训练接口 (POST /api/v3/tts/voice_clone)   │
│                                           │                             │
│                                           ▼ 产出持久唯一标识             │
│                                      speaker_id                         │
│                                           │                             │
│                                           ▼ 登记入库                    │
│                                [前端/业务音色字典 JSON]                 │
└───────────────────────────────────────────┬─────────────────────────────┘
                                            │ 仅传递轻量 ID 映射
┌───────────────────────────────────────────▼─────────────────────────────┐
│                       数据面（Data Plane · 核心网关）                    │
│                                                                         │
│   客户端请求 ──► POST /v1/audio/speech { model, input, voice: "S_xxx" } │
│                                           │                             │
│                                           ▼ 极速鉴权与配额扣减           │
│                                     业务网关                            │
│                                           │                             │
│                                           ▼ 携带 speaker: "S_xxx" 转发   │
│                       火山大模型 TTS 3.0 接口 (POST /api/v3/text-to-speech/{voice_id}/streaming)│
│                                           │                             │
│                                           ▼ JSONL 实时流式音频          │
│   客户端播放 ◄──────────────────────── 音频二进制 / JSON / NDJSON       │
└─────────────────────────────────────────────────────────────────────────┘
```

### 职责边界定义

| 维度 | 控制面（音色训练与管理） | 数据面（音频生成） |
| :--- | :--- | :--- |
| **执行载体** | 音色注册接口（`volc-voice-clone` 任务适配器） | 核心网关（OpenAI 兼容 `/v1/audio/speech`） |
| **调用频率** | 极低频（按需录入、预制音色库、运营上架） | 极高频（业务实时调用、高并发语音合成） |
| **执行人员/主体**| 内部管理员、运营人员、授权终端用户 | 终端用户、第三方 API 客户端、业务 Agent |
| **处理数据** | 音频样本（Base64，10~60 秒，≤10MB） | 纯文本 / SSML、语音表达指令、音频输出流 |
| **交付产物** | `omnimux_*` 唯一音色代号 + 1 小时试听样音 | 最终合成语音流（MP3/PCM/OGG Opus）及可选字词字幕 |
| **计费与权限** | 按次计费（绑定 `user_id` 所有权隔离） | 按输入字符计费（汉字=2 字符，其余=1 字符） |

---

## 3. 火山引擎 TTS 3.0 官方接口对应关系（域名：`https://voice.cn-beijing.volces.com`）

| 业务环节 | 官方接口路径 | 核心输入 | 核心输出 | 官方文档 |
| :--- | :--- | :--- | :--- | :--- |
| **音色注册/克隆**（真实音频） | `POST /api/v3/tts/voice_clone` | `audio.data` (Base64) + `speaker_id="custom_speaker_id"` + `custom_speaker_id` | `speaker_id` + `status` + 1小时临时 `demo_audio` | `seed-tts-3-0-voice-cloning-register` |
| **音色查询** | `POST /api/v3/tts/get_voice` | `speaker_id="custom_speaker_id"` + `custom_speaker_id` | `status`（`2`/`4` 可合成）+ `demo_audio` | `seed-tts-3-0-voice-query` |
| **TTS 3.0 单向流式合成**（在线消费） | `POST /api/v3/text-to-speech/{voice_id}/streaming` | `text` / `ssml` + `model_id="doubao-seed-tts-3.0"` + 路径 `voice_id` | JSONL 事件流（`ready` / `audio` / `alignment` / `final`） | `seed-tts-3-0-http-streaming` |

---

## 4. 运维与落地规范

### 4.1 控制面：音色克隆与产出
1. 音频准备：使用标准高质量录音（推荐 WAV/MP3，10~60 秒，≤10MB，背景安静）；
2. 通过网关 `volc-voice-clone` 任务提交 Base64 音频与 `omnimux_*` 自定义代号（可选 `prompt_text` / `prosody_boost`）；
3. 网关自动调用 `https://voice.cn-beijing.volces.com/api/v3/tts/voice_clone` 与 `get_voice` 完成训练与状态轮询；
4. 若需持久保存试听样音，应在 1 小时有效期内自行下载归档。

### 4.2 数据面：网关透明透传
1. 前端/客户端直接读取音色字典（`omnimux voices --model seed-tts-3.0`）或已注册的 `omnimux_*` 音色 ID；
2. 客户端向网关发起标准请求：
   ```json
   {
     "model": "seed-tts-3.0",
     "input": "大家好，欢迎来到本期播客节目。",
     "voice": "multi_male_shenyeboke_v3_uranus_bigtts"
   }
   ```
3. 网关校验 `omnimux_*` 所有权归属后，将 `voice` 编码为单段路径参数调用火山 TTS 3.0 HTTP 流式接口；
4. 网关**不需要**保存任何音色音频，做到“零音色文件存储网关”。
