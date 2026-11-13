# 火山引擎音频生成 API (seed-audio-1.0) 接入系统架构设计与任务分解

## 一、 系统架构与核心技术选型决策

### 1. 方案权衡：复用既有 Channel Type 17 (`VolcEngine`)
- **决策结论**：**复用既有 Channel Type 17 (`VolcEngine`)，不新建独立渠道类型**。
- **决策依据**：
  1. **商用账户与配置一致性**：火山引擎的控制台账号与 API Key 是统一的。复用 Type 17 使运营配置一个火山渠道即可同时支持豆包文本、图像、传统 TTS 以及最新 `seed-audio-1.0`，避免配置冗余。
  2. **模型级动态路由（Sub-router Pattern）**：OmniMux 的 Adaptor 机制原生支持在同一 Adaptor 内部根据 Model 名称分流。在 `relay/channel/volcengine/adaptor.go` 中判定模型为 `seed-audio-*` 时走 HTTP REST v3 管道，其他模型保持旧版 WebSocket 二进制管道。
  3. **高内聚与极小侵入**：无须在系统枚举、全局路由、管理台前端新增渠道类型，变动全部内聚在 `relay/channel/volcengine/` 模块内部。

### 2. 请求流转与协议转换设计
客户端通过 OpenAI 标准规范调用 `POST /v1/audio/speech`，系统采用**双轨解析映射规范**：
1. **基础参数自动转换**：
   - `request.Input` $\rightarrow$ `text_prompt`
   - `request.Model` $\rightarrow$ `model`（`seed-audio-1.0`）
   - `request.Voice` $\rightarrow$ 若无显式传入 `references`，作为预置音色填充进 `references[0].speaker`
   - `request.Speed` $\rightarrow$ `audio_config.speech_rate`
   - `request.ResponseFormat` $\rightarrow$ `audio_config.format`（默认 `mp3`，支持 `wav`, `pcm`, `ogg_opus`）
2. **高级克隆参数承载（References & AudioConfig）**：
   - 在 `relaykit/dto/audio.go` 的 `AudioRequest` 中扩展高级字段，同时支持从 `request.Metadata` 或顶层 JSON 解包；
   - 客户端可在调用时直接传入完整音色克隆参数 `references`（包含 `audio_url`/`audio_data` 与 `image_url`）及水印配置 `watermark`，无损透传至火山引擎 HTTP v3 接口。

### 3. 响应处理设计：二进制音频流模式 vs 结构化 JSON 模式
根据客户端请求特征，支持自适应双模响应：
1. **模式 A：标准二进制音频流响应（默认模式）**
   - **触发条件**：常规 OpenAI 客户端或未指定 `Accept: application/json`；
   - **行为**：将上游 Base64 音频内容解码为二进制字节，以 `Content-Type: audio/mpeg`（或相应格式）直接写入 HTTP 响应体；
   - **元数据透传**：通过自定义 Header 透传关键上下文：
     - `X-Audio-Duration: <duration>`
     - `X-Audio-Original-Duration: <original_duration>`
     - `X-Audio-Url: <url>`（上游生成的 2 小时有效试听/下载直链）。
2. **模式 B：结构化 JSON 模式（高级能力模式）**
   - **触发条件**：客户端请求头包含 `Accept: application/json` 或入参 `response_format` 为 `json`/`verbose_json`；
   - **行为**：直接透传上游完整 JSON 数据（包含 base64, duration, original_duration, url, subtitle 字级时间戳），方便视频对口型、AI 剪辑业务消费。

### 4. 鉴权兼容层设计
火山引擎 HTTP v3 端点采用单头鉴权 `X-Api-Key: <your_api_key>`：
- **兼容已有配置**：设计鉴权提取器 `extractVolcengineApiKey`；
  - 若渠道配置的 Key 为传统格式 `appid|token`，自动提取第 2 段（`token`）作为 `X-Api-Key`；
  - 若为纯 API Key，则直接设为 `X-Api-Key`；
- 无须用户修改已有的火山渠道配置即可开箱即用。

### 5. 计费与结算集成
- **时长转化为 AudioTokens**：
  从上游 JSON 出参中提取计费基准时长（优先取 `original_duration`，缺省取 `duration`）；
  遵循 OpenAI TTS 计费规范：
  $$\text{AudioTokens} = \text{QuotaRound}\left(\lceil\text{duration}\rceil / 60.0 \times 1000\right)$$
- **自动接入既有管道**：
  构建 `dto.Usage` 并将计算出的 Token 填入 `usage.CompletionTokenDetails.AudioTokens`；
  在 `relay/audio_handler.go` 中，只要 `AudioTokens > 0`，系统将自动流转至 `service.PostAudioConsumeQuota`，精准按照渠道设置的模型音频倍率进行结算与多退少补。

---

## 二、 任务分解与实现顺序（WBS）

| 任务编号 | 任务名称 | 包含源文件（每任务 ≥ 3 文件） | 前置依赖 | 自测验收点 | 优先级 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **T01** | **项目基础设施与数据传输契约** | 1. `relay/channel/volcengine/constants.go`<br>2. `relaykit/dto/audio.go`<br>3. `relay/channel/volcengine/seed_audio_dto.go` [新建] | 无 | 1. `cd relaykit && GOWORK=off go build ./...` 独立编译通过<br>2. 根目录 `go build ./relay/...` 编译无误 | **P0** |
| **T02** | **火山引擎适配器路由分发与请求转换** | 1. `relay/channel/volcengine/adaptor.go`<br>2. `relay/channel/volcengine/seed_audio.go` [新建]<br>3. `relay/channel/volcengine/seed_audio_auth.go` [新建] | T01 | 1. 模型前缀判断准确路由至 HTTP v3<br>2. 鉴权提取器覆盖纯 Key 与 `appid\|token` 解包<br>3. 正确生成符合官方格式的 JSON Body | **P0** |
| **T03** | **响应解析、双模输出与音频计费集成** | 1. `relay/channel/volcengine/seed_audio_response.go` [新建]<br>2. `relay/channel/volcengine/tts.go`<br>3. `relay/audio_handler.go` | T02 | 1. 客户端可正确接收二进制音频及 `X-Audio-Url` 等 Header<br>2. 指定 JSON 时输出完整 subtitle 结构<br>3. `Usage.AudioTokens` 精准按秒换算触发扣费 | **P0** |
| **T04** | **单元测试验证、文档同步与最终验收** | 1. `relay/channel/volcengine/seed_audio_test.go` [新建]<br>2. `FORK_CUSTOMIZATIONS.md`<br>3. `docs/architecture/volcengine-seed-audio-architecture-zh.md` | T03 | 1. `go test -v ./relay/channel/volcengine` 全量通过<br>2. 全仓 `go build ./...` 编译成功<br>3. Fork 定制文档准确记录 | **P0** |

---

## 三、 受保护路径清单

实施受保护核心变更前需取得授权：
1. `relay/channel/volcengine/` (`constants.go`, `adaptor.go`, `tts.go` 及新建 `seed_audio_*.go`)
2. `relaykit/dto/audio.go`
3. `relay/audio_handler.go`
