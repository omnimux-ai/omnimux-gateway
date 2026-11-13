# grok2api 渠道供应商接口文档

- 服务: `grok2api` · 版本 `v3.1.5`
- 来源: https://grok2api.omnimux.ai/docs/chat/completions
- 上游仓库: <https://github.com/chenyme/grok2api>
- Base URL: `https://grok2api.omnimux.ai/v1`（本地调试 `http://127.0.0.1:8000/v1`）
- 鉴权: `Authorization: Bearer g2a_...`
- 说明: chat/messages 使用 x-api-key + anthropic-version: 2023-06-01
- 接口总数: **16**

## Chat

### POST `/v1/chat/completions` — Chat completions

> Build / Web Chat Completions，支持流式响应、图片输入与函数工具。

- 能力: `chat, responses`
- Base URL: `https://grok2api.omnimux.ai/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `messages` | 是 | OpenAI Chat Completions 消息数组，支持文本、图片输入与工具结果。 |
| `stream` | 否 | 是否以 SSE 增量返回；默认 false。 |
| `max_completion_tokens` | 否 | 限制本次请求可生成的最大 Completion Token 数。 |
| `tools` | 否 | 标准函数工具定义，按 Provider 能力原生转发或转换。 |
| `tool_choice` | 否 | 控制工具选择，可使用 auto、none、required 或指定函数。 |

**实现说明**

- 对话图片会先完成校验与上传，再绑定到本次选择的账号；不会在流开始后切换账号。
- 工具调用会规范化为 OpenAI tool_calls；不支持的组合会返回明确参数错误。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "https://grok2api.omnimux.ai/v1/chat/completions" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "your-enabled-model",
  "messages": [
    {
      "role": "system",
      "content": "You are a concise assistant."
    },
    {
      "role": "user",
      "content": "Explain HTTP streaming."
    }
  ],
  "stream": false
}'
```

**响应示例**

```json
{
  "id": "chatcmpl_example",
  "object": "chat.completion",
  "model": "grok-4.6",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "HTTP streaming sends response data incrementally."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 24,
    "completion_tokens": 12,
    "total_tokens": 36
  }
}
```

### POST `/v1/responses` — Responses

> 创建文本、工具调用、结构化输出或流式 Responses 请求。

- 能力: `chat, responses`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `input` | 是 | 字符串或标准 Responses input item 数组。 |
| `instructions` | 否 | 独立于 input 的系统级指令。 |
| `stream` | 否 | 是否以 SSE 增量返回；默认 false。 |
| `store` | 否 | Build/Web 可保存 Response 状态以供查询或连续会话；Console 仅支持无状态请求。 |
| `previous_response_id` | 否 | Build/Web 使用上一轮 Response ID 继续同一条有状态会话链；Console 不支持该字段。 |
| `tools` | 否 | 标准函数工具定义，按 Provider 能力原生转发或转换。 |

**实现说明**

- store 与 previous_response_id 会把会话链绑定到原账号，避免跨账号破坏上游状态。
- 仅 Grok Build 支持 /responses/compact；Grok Web 与 Grok Console 会返回明确的不支持错误。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/responses" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-4.6",
  "input": "Explain HTTP streaming.",
  "store": false,
  "stream": false
}'
```

**响应示例**

```json
{
  "id": "resp_example",
  "object": "response",
  "status": "completed",
  "model": "grok-4.6",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "status": "completed",
      "content": [
        {
          "type": "output_text",
          "text": "HTTP streaming sends response data incrementally."
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 18,
    "output_tokens": 12,
    "total_tokens": 30
  }
}
```

### POST `/v1/messages` — Messages

> Build / Web Anthropic Messages，支持流式响应、图片与工具调用。

- 能力: `chat, responses`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `x-api-key: g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `max_tokens` | 是 | Anthropic Messages 要求的最大输出 Token 数。 |
| `messages` | 是 | Anthropic Messages 消息数组，支持文本、图片与 tool_result 内容块。 |
| `system` | 否 | 系统提示词，可使用字符串或标准文本内容块数组。 |
| `stream` | 否 | 是否以 SSE 增量返回；默认 false。 |
| `tools` | 否 | Anthropic 格式的工具定义，转换为当前 Provider 的函数调用能力。 |
| `tool_choice` | 否 | 控制工具选择，可使用 auto、none、required 或指定函数。 |

**实现说明**

- 流式响应输出 Anthropic Messages 事件，不混用 OpenAI SSE 事件名。
- 仅适配当前聊天模型共有的 Messages 能力，Provider 不支持的扩展字段会被拒绝。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/messages" \
  -H "x-api-key: $GROK2API_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-4.6",
  "max_tokens": 1024,
  "system": "You are a concise assistant.",
  "messages": [
    {
      "role": "user",
      "content": "Explain HTTP streaming."
    }
  ],
  "stream": false
}'
```

**响应示例**

```json
{
  "id": "msg_example",
  "type": "message",
  "role": "assistant",
  "model": "grok-4.6",
  "content": [
    {
      "type": "text",
      "text": "HTTP streaming sends response data incrementally."
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 18,
    "output_tokens": 12
  }
}
```


## Image

### POST `/v1/images/generations` — Image generations

> Grok Web 图片生成，支持 n、宽高比、1k/2k、URL、Base64 和扩展流式输出。

- 能力: `image`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `prompt` | 是 | 用于生成或编辑图片的自然语言描述。 |
| `n` | 否 | 需要返回的图片数量；Web 会原样映射为上游 num_generations。 |
| `aspect_ratio` | 否 | 输出媒体的宽高比，例如 1:1、2:3 或 16:9。 |
| `resolution` | 否 | Console 图片支持 1k/2k；Web 图片生成忽略 resolution/quality，并由模型名称选择上游产品。 |
| `quality` | 否 | Console Image 2.0 的生成质量，可选 low 或 medium；默认 medium。Web 图片生成路由忽略该字段。 |
| `response_format` | 否 | 图片返回格式，支持 url 或 b64_json。 |
| `stream` | 否 | 是否使用图片生成扩展 SSE 流；默认 false。 |

**实现说明**

- 客户端通过 n 指定图片数量；Web 将其原样映射为上游 num_generations。
- 生成结果统一归档到媒体存储；url 返回网关资源地址，b64_json 返回编码后的图片内容。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/images/generations" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-imagine-image-lite",
  "prompt": "A minimal red chair in a bright studio",
  "n": 1,
  "response_format": "url"
}'
```

**响应示例**

```json
{
  "created": 1783860000,
  "data": [
    {
      "url": "http://127.0.0.1:8000/v1/media/images/example"
    }
  ]
}
```

### POST `/v1/images/edits` — Image edits

> Grok Web 图片编辑，使用官方 JSON 图片 URL 协议。

- 能力: `image_edit`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `prompt` | 是 | 用于生成或编辑图片的自然语言描述。 |
| `image / images` | 是 | 待编辑图片，使用 image 或 images 传入 URL、Data URL 或已支持的图片引用。 |
| `n` | 否 | 需要返回的图片数量；Web 会原样映射为上游 num_generations。 |
| `quality` | 否 | Console Image 2.0 的生成质量，可选 low 或 medium；默认 medium。Web 图片生成路由忽略该字段。 |
| `response_format` | 否 | 图片返回格式，支持 url 或 b64_json。 |

**实现说明**

- 图片编辑采用 JSON 请求体，不提供 multipart 兼容层。
- 远程图片会经过 SSRF、防体积超限和内容类型校验后再上传到上游。
- 生成结果统一归档到媒体存储；url 返回网关资源地址，b64_json 返回编码后的图片内容。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/images/edits" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-imagine-image-edit",
  "prompt": "Change the chair to black",
  "image": {
    "url": "https://example.com/chair.png"
  },
  "n": 1,
  "response_format": "url"
}'
```

**响应示例**

```json
{
  "created": 1783860000,
  "data": [
    {
      "url": "http://127.0.0.1:8000/v1/media/images/example"
    }
  ]
}
```


## Video

### POST `/v1/videos/generations` — Video generations

> 按 xAI 官方协议创建异步视频生成任务并返回 request_id。

- 能力: `video`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `prompt` | 否 | 视频描述；使用图片输入时可作为运动或镜头指令。 |
| `duration` | 否 | 视频时长（秒），范围 1–15。Console 路由中，grok-imagine-video 的参考图模式最长 10 秒；grok-imagine-video-1.5 最长 15 秒。 |
| `aspect_ratio` | 否 | 输出媒体的宽高比，例如 1:1、2:3 或 16:9。 |
| `resolution` | 否 | 视频输出分辨率，支持 480p/720p；仅上游模型 grok-imagine-video-1.5 在非参考图模式下支持 1080p。 |
| `image` | 否 | 可选首帧图片引用，用于图生视频；最多 1 张，不可与 reference_images 同时使用。 |
| `reference_images` | 否 | 可选参考图片数组，用于参考图视频；网关最多接收 8 张，Console 路由最多 7 张。每项提供 url 或 file_id，不可与 image 同时使用。 |
| `reference_audios` | 否 | 可选参考音频数组，最多 3 个内置 voice_id；可单独使用或与 reference_images 组合。该能力可能受上游团队权限控制；Console 未开通时上游返回 403。 |

**实现说明**

- 创建接口只返回 request_id；生成过程异步执行并持久化任务状态。
- 参考图视频走 /videos/generations + reference_images；只传 1 张也保持参考图语义，不会被当成首帧。网关最多接收 8 张，Console 路由最多 7 张；不可与 image 同用。参考图模式最高 720p。Console 的 grok-imagine-video 参考图模式最长 10 秒，reference_audios 还需相应团队权限。
- 视频接口严格使用当前官方字段，不保留旧项目的兼容别名。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/videos/generations" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-imagine-video-1.5",
  "prompt": "A paper airplane flying over a city",
  "duration": 8,
  "aspect_ratio": "16:9",
  "resolution": "720p"
}'
```

**响应示例**

```json
{
  "request_id": "video_example"
}
```

### POST `/v1/videos/edits` — Video edits

> 按 xAI 官方协议编辑已有视频，并返回 request_id。

- 能力: `video`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `prompt` | 是 | 描述希望如何修改原视频。 |
| `video` | 是 | 输入视频，支持 url 或本地上传后的 file_id。 |

**实现说明**

- 创建接口只返回 request_id；生成过程异步执行并持久化任务状态。
- 视频编辑与延长的实际路由必须指向 Console/grok-imagine-video；对外模型名可以自定义。
- 视频编辑与延长暂未返回可可靠结算的输入视频时长，因此保持可调用，并在请求审计中标记为未计费；不会消耗 API Key 的金额额度。
- 视频接口严格使用当前官方字段，不保留旧项目的兼容别名。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/videos/edits" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-imagine-video",
  "prompt": "Give the woman a silver necklace",
  "video": {
    "url": "https://example.com/source.mp4"
  }
}'
```

**响应示例**

```json
{
  "request_id": "video_edit_example"
}
```

### POST `/v1/videos/extensions` — Video extensions

> 按 xAI 官方协议延长已有视频，并返回 request_id。

- 能力: `video`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 是 | 模型页中已启用且当前存在可用账号支持的对外模型名称。 |
| `prompt` | 是 | 描述视频接下来应发生的内容。 |
| `video` | 是 | 输入视频，支持 url 或本地上传后的 file_id。 |
| `duration` | 否 | 延长片段时长，单位秒，范围 2 到 10，默认 6。 |

**实现说明**

- 创建接口只返回 request_id；生成过程异步执行并持久化任务状态。
- 视频编辑与延长的实际路由必须指向 Console/grok-imagine-video；对外模型名可以自定义。
- 视频编辑与延长暂未返回可可靠结算的输入视频时长，因此保持可调用，并在请求审计中标记为未计费；不会消耗 API Key 的金额额度。
- 视频接口严格使用当前官方字段，不保留旧项目的兼容别名。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/videos/extensions" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-imagine-video",
  "prompt": "The shot pans to an over the shoulder perspective.",
  "duration": 6,
  "video": {
    "url": "https://example.com/source.mp4"
  }
}'
```

**响应示例**

```json
{
  "request_id": "video_extend_example"
}
```

### GET `/v1/videos/{request_id}` — Get video

> 按 request_id 查询 pending、done 或 failed 状态及生成结果。

- 能力: `video`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `request_id` | 是 | 创建视频任务时返回的 request_id。 |

**实现说明**

- 使用创建接口返回的 request_id 轮询任务状态。
- 响应状态为 pending、done 或 failed；完成后 video 中包含结果信息。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X GET "http://127.0.0.1:8000/v1/videos/video_example" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json"
```

**响应示例**

```json
{
  "status": "done",
  "model": "grok-imagine-video-1.5",
  "progress": 100,
  "video": {
    "url": "https://example.com/generated.mp4",
    "duration": 8,
    "respect_moderation": true
  }
}
```


## Voice

### POST `/v1/tts` — Text to speech

> Console 文本转语音，返回音频二进制或带时间戳的 JSON 封装。

- 能力: `tts`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 语音模型公开 ID，例如 grok-voice-latest 或 grok-voice-think-fast-2.0。 |
| `text` | 是 | 需要合成的文本内容。 |
| `voice_id` | 否 | 音色 ID；可使用内置音色或自定义音色 voice_id。 |
| `language` | 是 | 语言代码，例如 en、zh；TTS 必填。 |
| `output_format` | 否 | 输出格式对象，可指定 codec、sample_rate、bit_rate。 |
| `speed` | 否 | 语速倍率；按上游支持范围校验。 |
| `with_timestamps` | 否 | 为 true 时返回含 base64 音频与时间戳的 JSON，而不是原始音频流。 |

**实现说明**

- Console 语音模型包括 grok-voice-latest、grok-voice-think-fast-2.0 与 grok-voice-think-fast-1.0。
- 默认响应为 audio/mpeg 等音频字节流；with_timestamps=true 时改为 JSON。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/tts" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-voice-latest",
  "text": "Hello from Grok voice.",
  "voice_id": "eve",
  "language": "en",
  "output_format": {
    "codec": "mp3"
  }
}'
```

**响应示例**

```json
{
  "content_type": "audio/mpeg",
  "note": "Default responses return raw audio bytes. with_timestamps=true returns a JSON envelope."
}
```

### POST `/v1/audio/speech` — OpenAI speech

> OpenAI 兼容文本转语音接口，接受 input/voice/response_format 并返回音频字节。

- 能力: `tts`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 语音模型公开 ID，例如 grok-voice-latest 或 grok-voice-think-fast-2.0。 |
| `input` | 是 | OpenAI 风格待合成文本，对应原生 text 字段。 |
| `voice` | 否 | OpenAI 音色名或 Console voice_id；常见 OpenAI 音色会自动映射。 |
| `response_format` | 否 | 输出格式，如 mp3、opus、aac、flac、wav、pcm。 |
| `speed` | 否 | 语速倍率；按上游支持范围校验。 |
| `language` | 否 | 语言代码，例如 en、zh；OpenAI 兼容接口未提供时默认 en。 |

**实现说明**

- 兼容 OpenAI /v1/audio/speech：input→text，voice→voice_id，response_format→codec；也可继续传 language 等扩展字段。
- Console 语音模型包括 grok-voice-latest、grok-voice-think-fast-2.0 与 grok-voice-think-fast-1.0。
- 默认响应为 audio/mpeg 等音频字节流；with_timestamps=true 时改为 JSON。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/audio/speech" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-voice-latest",
  "input": "Hello from Grok voice.",
  "voice": "alloy",
  "response_format": "mp3",
  "speed": 1,
  "language": "en"
}'
```

**响应示例**

```json
{
  "content_type": "audio/mpeg",
  "note": "Returns raw audio bytes compatible with OpenAI speech clients."
}
```

### POST `/v1/audio/tasks` — OpenAI audio tasks

> OpenAI 风格音频任务兼容路径，默认返回音频字节。

- 能力: `tts`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 语音模型公开 ID，例如 grok-voice-latest 或 grok-voice-think-fast-2.0。 |
| `input` | 是 | OpenAI 风格待合成文本，对应原生 text 字段。 |
| `voice` | 否 | OpenAI 音色名或 Console voice_id；常见 OpenAI 音色会自动映射。 |
| `response_format` | 否 | 输出格式，如 mp3、opus、aac、flac、wav、pcm。 |
| `speed` | 否 | 语速倍率；按上游支持范围校验。 |
| `language` | 否 | 语言代码，例如 en、zh；OpenAI 兼容接口未提供时默认 en。 |

**实现说明**

- 部分客户端使用 /v1/audio/tasks；本网关按 speech 同步处理，默认返回音频字节。
- Console 语音模型包括 grok-voice-latest、grok-voice-think-fast-2.0 与 grok-voice-think-fast-1.0。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/audio/tasks" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-voice-latest",
  "input": "Hello from Grok voice.",
  "voice": "alloy",
  "response_format": "mp3",
  "language": "en"
}'
```

**响应示例**

```json
{
  "content_type": "audio/mpeg",
  "note": "Compatibility path that returns raw audio bytes by default."
}
```

### POST `/v1/audio/transcriptions` — OpenAI transcriptions

> OpenAI 兼容语音转文字接口，复用现有 STT 能力。

- 能力: `stt`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 转写模型公开 ID，通常为 grok-stt。 |
| `file` | 否 | multipart 上传的音频文件字段名 file。 |
| `url` | 否 | 可公开访问的音频 URL；与 file 二选一。 |
| `language` | 否 | 语言代码，例如 en、zh；TTS 必填。 |
| `response_format` | 否 | 响应格式：json（默认）、verbose_json 或 text；未实现的 srt/vtt 会明确拒绝。 |

**实现说明**

- 兼容 OpenAI /v1/audio/transcriptions，支持 json、verbose_json 与 text 响应，底层复用 /v1/stt；prompt、非零 temperature 和 timestamp_granularities 暂无法无损转换，将明确拒绝。
- 支持 multipart 文件上传或 JSON url 引用；至少提供其中一种音频输入。
- 转写模型使用 grok-stt；词级时间戳与说话人分离按请求参数返回。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/audio/transcriptions" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-stt",
  "url": "https://example.com/sample.wav",
  "language": "en"
}'
```

**响应示例**

```json
{
  "text": "Hello from Grok voice."
}
```

### GET `/v1/tts/voices` — List voices

> 列出 Console 当前可用的内置音色。

- 能力: `tts`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 可选模型查询参数，用于选择音色列表所对应的语音模型。 |

**实现说明**

- GET /v1/tts/voices 返回内置音色；GET /v1/tts/voices/{voice_id} 可查询单个音色。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X GET "http://127.0.0.1:8000/v1/tts/voices?model=grok-voice-latest" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json"
```

**响应示例**

```json
{
  "voices": [
    {
      "voice_id": "eve",
      "name": "Eve",
      "language": "en"
    },
    {
      "voice_id": "ara",
      "name": "Ara",
      "language": "en"
    }
  ]
}
```

### POST `/v1/stt` — Speech to text

> Console 语音转文字，支持文件上传或音频 URL，可返回词级时间戳。

- 能力: `stt`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | 转写模型公开 ID，通常为 grok-stt。 |
| `file` | 否 | multipart 上传的音频文件字段名 file。 |
| `url` | 否 | 可公开访问的音频 URL；与 file 二选一。 |
| `language` | 否 | 语言代码，例如 en、zh；TTS 必填。 |
| `format` | 否 | 是否返回格式化文本与词级时间戳。 |
| `diarize` | 否 | 是否启用说话人分离。 |
| `keyterm` | 否 | 提升识别优先级的关键词，可重复提交。 |

**实现说明**

- 支持 multipart 文件上传或 JSON url 引用；至少提供其中一种音频输入。
- 对 /v1/stt 发起 WebSocket Upgrade 可进行实时转写，客户端发送音频帧并接收 JSON 事件。
- 转写模型使用 grok-stt；词级时间戳与说话人分离按请求参数返回。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X POST "http://127.0.0.1:8000/v1/stt" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
  "model": "grok-stt",
  "url": "https://example.com/sample.wav",
  "language": "en",
  "format": true
}'
```

**响应示例**

```json
{
  "text": "Hello from Grok voice.",
  "language": "en",
  "duration": 1.84,
  "words": [
    {
      "text": "Hello",
      "start": 0,
      "end": 0.42
    }
  ]
}
```

### GET `/v1/realtime` — Realtime websocket

> Realtime 语音 WebSocket，由网关代持 Console 账号完成上游鉴权与双向转发。

- 能力: `realtime`
- Base URL: `http://127.0.0.1:8000/v1`
- 鉴权: `Authorization: Bearer g2a_...`
- 示例语言: `curl`

**请求参数**

| 参数 | 必填 | 说明 |
|---|---|---|
| `model` | 否 | WebSocket 连接的 model 查询参数。 |
| `Upgrade` | 是 | 必须携带标准 WebSocket Upgrade 头；普通 HTTP GET 会被拒绝。 |

**实现说明**

- 连接后按官方 Realtime 事件协议交换 session、音频缓冲与响应事件。
- 推荐通过本网关的 /v1/realtime WebSocket 代理连接，由 Console 账号完成上游 DPoP 鉴权。
- 服务端 WebSocket 客户端需在 Upgrade 请求中携带 Authorization Bearer 或 X-API-Key；网关不会向调用方暴露上游 Console 凭据。
- TTS 按输入字符数预留并结算；REST 与流式 STT 按成功响应返回的实际时长结算。Realtime 当前标记为未计费，不消耗 API Key 金额额度。

**请求示例**

```
export GROK2API_API_KEY="g2a_your_api_key"

curl -X GET "http://127.0.0.1:8000/v1/realtime?model=grok-voice-latest" \
  -H "Authorization: Bearer $GROK2API_API_KEY" \
  -H "Content-Type: application/json"
```

**响应示例**

```json
{
  "type": "session.created",
  "session": {
    "model": "grok-voice-latest"
  }
}
```

