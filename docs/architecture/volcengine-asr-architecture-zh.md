# 火山引擎豆包语音识别大模型（Auc BigModel ASR）开发与接入指南

> **技术接入边界（2026-10-08 标识）：** 本文保留语音转写技术指南与原有状态陈述，本次不重新验收接口、定价或生产配置；“生产就绪”不能据此推导当前消费服务范围。历史全自动切片、配音或营销流水线不属于当前默认承诺，不得恢复 AI UGC 团队、无订阅、发布复盘全链路、旧页面路径或用户手选模型/分组。现行依据见 [品牌规范](../brand/README.md) 与 [落地页文案与实现](../brand/landing-page-copy.md)；请求示例、鉴权、输入格式及素材契约保持不变。

**文档代号**：`docs/architecture/volcengine-asr-architecture-zh.md`  
**模块分类**：Audio Transcription / Speech-to-Text (STT)  
**维护团队**：OmniMux Core Team  
**当前状态**：生产就绪（Production Ready）  

---

## 1. 架构总览与产品定位

OmniMux 当前聚焦 **AI 角色生成与视频复刻**（现行依据见 [品牌规范](../brand/README.md)）。本文记录**火山引擎豆包语音 · 录音文件识别大模型标准版（Auc BigModel ASR）**的中枢接入与语音转写契约；接口存在不等于当前产品自动提供文案拆解、多语种重配音、切片、发布或复盘。

### 核心能力特性：
1. **100% 兼容 OpenAI Whisper 标准协议**：对齐标准 `POST /v1/audio/transcriptions` 接口规范，既有 OpenAI SDK、LangChain、LlamaIndex 及剪辑客户端**零代码改造即可平替迁移**；
2. **URL-first 零拷贝直传管道**：支持入参直接提供公网音频直链（`url` / `audio_url`），由上游火山引擎直连抓取，彻底免除网关本地磁盘 I/O 读写与网络二次上传带宽瓶颈；
3. **Context-aware 自适应短轮询调度引擎**：上游采用异步 Submit/Query 架构，网关内部实现指数退避短轮询调度引擎（$300\text{ms} \to 450\text{ms} \to 675\text{ms} \to 1000\text{ms}$，超时 60s），常规短视频在 1.5~3 秒内以 HTTP 200 阻塞直出；同时深度绑定客户端连接生命周期，客户端主动断连时立即响应中断（499），严防 Goroutine 泄漏与无效计费；
4. **高精度 5 模格式转换管道**：原生支持 `json`（标准文本）、`text`（纯文本流）、`verbose_json`（毫秒换算浮点秒，对齐 segments 数组）、`srt`（标准 SubRip 外挂字幕）与 `vtt`（标准 WebVTT 网页字幕）；
5. **智能短视频口播清洗**：默认启用语义顺滑（`enable_ddc=true`，剔除口吃、语气词）、逆文本标准化（`enable_itn=true`，金额数字规范化）与标点预测（`enable_punc=true`）。

---

## 2. API 接口调用规范

- **请求端点**：`POST https://omnimux.ai/v1/audio/transcriptions`
- **认证方式**：`Authorization: Bearer <OMNIMUX_API_KEY>`
- **请求格式**：`application/json`（推荐 URL 直传）或 `multipart/form-data`（支持文件上传）

### 2.1 请求入参说明

| 参数名 | 类型 | 必填 | 默认值 | 描述 |
| :--- | :--- | :---: | :---: | :--- |
| `model` | string | 是 | `doubao-asr-bigmodel` | 模型名称，可选 `doubao-asr-bigmodel`（豆包2.0，推荐）、`seedasr-auc`、`bigasr-auc`（豆包1.0） |
| `url` / `audio_url` | string | 否* | 空 | 公网可访问的音频/视频链接（支持 mp3, wav, m4a, ogg, pcm, aac, flac 等）。**URL-first 模式下必填** |
| `file` | file | 否* | 空 | 音频文件二进制流（与 `url` 二选一） |
| `response_format` | string | 否 | `json` | 期望的响应格式，可选 `json`, `text`, `verbose_json`, `srt`, `vtt`, `doubao_json` |
| `language` | string | 否 | 自动识别 | 语种提示代码（如 `zh-CN`, `en-US`, `ja-JP`, `es-MX` 等） |
| `prompt` | string | 否 | 空 | 专有热词与语境提示词（自动映射至豆包大模型语境上下文配置） |
| `enable_ddc` | bool | 否 | `true` | 是否启用语义顺滑（自动剔除口吃、重复词与无意义语气词） |
| `enable_itn` | bool | 否 | `true` | 是否启用逆文本标准化（将口播“二十九块九”转为“29.9元”） |
| `enable_punc` | bool | 否 | `true` | 是否启用智能标点预测 |
| `enable_speaker_info` | bool | 否 | `false` | 是否启用说话人角色分离 |
| `ssd_version` | string | 否 | `"300"` | 说话人分离场景模型版本（`"200"` 适合短对话，`"300"` 适合会议长音频声纹匹配） |
| `enable_emotion_detection` | bool | 否 | `false` | 是否启用情绪检测（在 verbose_json 的 segments 中透出情绪标签） |

---

## 3. 典型代码调用示例

### 3.1 基础 JSON 文案提取（URL-first 直传）
```bash
curl -X POST "https://omnimux.ai/v1/audio/transcriptions" \
  -H "Authorization: Bearer sk-omnimux-xxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-asr-bigmodel",
    "url": "https://lf3-static.bytednsdoc.com/obj/eden-cn/lm_hz_ihsph/ljhwZthlaukjlkulzlp/console/bigtts/zh_female_cancan_mars_bigtts.mp3",
    "response_format": "json"
  }'
```
**返回**：
```json
{
  "text": "刚刚还在想你怎么还不来找我聊天，你就来了，真是心有灵犀啊！"
}
```

### 3.2 自动生成 SRT 剪辑字幕（毫秒级精准打轴）
```bash
curl -X POST "https://omnimux.ai/v1/audio/transcriptions" \
  -H "Authorization: Bearer sk-omnimux-xxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-asr-bigmodel",
    "url": "https://example.com/ecommerce_video.mp4",
    "response_format": "srt"
  }'
```
**返回（标准 SubRip 字幕格式，毫秒逗号分隔）**：
```text
1
00:00:00,480 --> 00:00:05,880
刚刚还在想你怎么还不来找我聊天，你就来了，真是心有灵犀啊！
```

### 3.3 详细分句与说话人、情绪结构化解析（Verbose JSON）
```bash
curl -X POST "https://omnimux.ai/v1/audio/transcriptions" \
  -H "Authorization: Bearer sk-omnimux-xxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-asr-bigmodel",
    "url": "https://example.com/podcast.mp3",
    "response_format": "verbose_json",
    "enable_speaker_info": true,
    "enable_emotion_detection": true
  }'
```
**返回**：
```json
{
  "task": "transcribe",
  "language": "chinese",
  "duration": 6.312,
  "text": "刚刚还在想你怎么还不来找我聊天，你就来了，真是心有灵犀啊！",
  "segments": [
    {
      "id": 0,
      "seek": 480,
      "start": 0.48,
      "end": 5.88,
      "text": "刚刚还在想你怎么还不来找我聊天，你就来了，真是心有灵犀啊！",
      "tokens": [],
      "temperature": 0.0,
      "avg_logprob": 0.0,
      "compression_ratio": 1.0,
      "no_speech_prob": 0.0,
      "speaker": "speaker_0",
      "emotion": "happy"
    }
  ]
}
```

---

## 4. 商业计费与配平规范

| 计价维度 | 标准数值 | 说明 |
| :--- | :--- | :--- |
| **零售定价** | **$0.003 USD / 分钟** | 约 $0.00005/秒，按音频实际秒数向上取整计算 |
| **积分定价** | **0.03 积分 / 分钟** | 对齐统一标准：1 USD = 10 积分 = 500,000 raw quota |
| **Raw Quota** | **1,500 raw quota / 分钟** | 1 积分 = 50,000 raw quota |
| **AudioTokens 换算** | **1,000 tokens / 分钟** | $\text{Tokens} = \lceil \frac{\text{DurationSeconds}}{60.0} \times 1000 \rceil$ |
| **模型倍率** | **`AudioRatio = 1.5`** | $1000\text{ tokens} \times 1.5 \times 1.0 = 1500\text{ raw quota}$ 完美配平 |

---

## 5. 生产环境运维与排障字典

### 5.1 渠道配置要求（Channel Type 45）
- **渠道类型**：`45` (VolcEngine)
- **Base URL**：`https://openspeech.bytedance.com`
- **支持模型**：必须包含 `seed-audio-1.0,doubao-asr-bigmodel,seedasr-auc,bigasr-auc`
- **分组**：`default,auto`
- **API Key**：必须是火山引擎**豆包语音控制台**专属 API Key（格式通常为 UUID 或标准 36 位密钥，严禁使用火山方舟 Ark Key）。

### 5.2 状态码与异常速查字典

| 状态码 | 错误提示 / 场景 | 根因说明与解决方案 |
| :--- | :--- | :--- |
| **400** | `audio url is required for Auc BigModel ASR` | 请求未提供音频直链。使用 URL 直传时请传入 `url` 或 `audio_url` |
| **502** | `volcengine asr submit returned status 45000010` | 火山引擎 API Key 无效或未开通语音识别服务。请检查渠道密钥配置 |
| **502** | `volcengine asr poller failed with code 55000000` | 火山引擎服务端处理异常（如音频损坏或格式无法解码） |
| **499** | `context canceled` | 客户端在任务识别完成前主动断开连接，网关安全退出轮询并释放资源 |
| **504** | `polling asr task timeout after 60s` | 长音频处理超过 60 秒同步等待阈值。超长音频建议使用分段切片调用 |
