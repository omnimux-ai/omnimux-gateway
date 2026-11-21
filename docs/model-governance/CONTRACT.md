# 模型档案契约（Model Spec Contract）

> 目的：把"每个模型到底支持什么、限制是什么、依据是什么"变成**落盘的、可复用的、可被门禁校验的**事实，
> 而不是靠记忆、上下文或临时搜索。模型规格很少变，所以一份档案可以被反复使用。

本文件是网关仓的模型治理入口。新工作树可直接按本文件操作。

## 一、三条铁律

1. **不得编造**。任何写进档案的数字（张数、时长、分辨率、大小、上下文长度、速率）都必须有
   `sources[]` 里的官方出处（url + 原文引述 + 核验日期）。查不到就写 `"status": "unverified"`，
   **绝不允许用"看起来合理"的数字填空**。
2. **一模型一文件**，落盘持久化。路径：`docs/model-governance/specs/<model_id>.json`。
   文件是唯一事实来源；聊天记录、上下文、记忆都不是。
3. **口径可核对**。`sources[].quote` 必须是出处里的原文片段（可含少量上下文），
   不是转述、不是博客、不是第三方教程。第三方只能作为线索，最终必须回到厂商官方页。
   出处有两种，按对象选：
   - **厂商模型** → 官方页面 URL（`https://...`），引文必须在该页面上真实存在。
   - **我们自己的接口**（`family: "data"`，或任何以本仓为准的事实）→ 写成 `repo:<仓内相对路径>`，
     例如 `repo:docs/ops/tikhub-social-models-zh.md`。门禁会**打开该文件核对引文是否真的存在**，
     所以这类出处比 URL 更硬，不是更松。

## 二、通用必填字段

```jsonc
{
  "model_id": "gpt-image-2.5",          // 我们的公开 ID，与目录一致
  "family": "image",                    // image | video | audio | llm | data
  "vendor": "OpenAI",                   // 面向用户展示的品牌平台
  "vendor_model_id": "gpt-image-1",     // 厂商侧真实 ID（若已知）
  "capability": "图像生成",
  "operations": ["text_to_image", "image_to_image"],   // 该模型实际支持的操作
  "status": "documented",               // documented | partial | unverified
  "checked_at": "2026-09-14",
  "sources": [
    {
      "claim": "单次最多 16 张参考图",
      "url": "https://...",
      "quote": "You can provide up to 16 images",
      "retrieved_at": "2026-09-14"
    }
  ],
  "notes": "免费文本，写清未确认项与已知坑"
}
```

- `status` 语义：
  - `documented` —— 关键限制都有官方出处。
  - `partial` —— 部分有出处，其余明确标注为未知（写进 `unknowns[]`）。
  - `unverified` —— 未找到任何官方出处；**此状态必须在档案里写明原因**。
- 未知项写进 `unknowns: ["..."]`，不要省略。

### `null` 的两种含义（必须分清）

字段为 `null` 时有两种完全不同的情况，混淆会让人"修错"：

| 含义 | 写法 | 例子 |
| --- | --- | --- |
| **厂商没有公布** | `null` + 该字段写进 `unknowns[]` | xAI 不公布每个模型的输出上限 |
| **厂商明确说无限制** | `null` + `notes` 里写明"官方说明无上限"，**且不进 `unknowns[]`** | grok-4.6 官方发布说明写"no text output limit" |

所以 `null` **不等于**"有缺口"：判断依据是它有没有出现在 `unknowns[]` 里。

### 出处的三级效力

1. **厂商官方页**（首选）：模型卡、模型列表、API 文档。
2. **厂商平台级官方说明**（可用，但必须标注）：例如"所有在役 Claude 模型均支持提示缓存"——
   这是官方口径，可以支撑某模型的能力字段，但 `claim` 里必须写明**它来自平台级说明，而非该模型页自述**，
   让人一眼能看出这条的效力层级。
3. **第三方页面**（只能当线索）：博客、教程、聚合站。**不得**作为限制数字的出处；
   只能用来找到官方页，或在官方页拿不到正文时作为线索并在 `notes` 明确标注为二手。

### 退役 / 别名 ID 的处理

目录里的 ID 可能是**厂商已退役的旧名**或**兼容别名**（例：`deepseek-v4-flash` 现网名叫 `deepseek-flash`，
旧名被官方"暂时路由"到新模型）。这类档案：

- 照常记录**官方声明过的别名路由关系**与数值，`status` 用 `partial`；
- 在 `notes` 里写明"这是别名/退役 ID，官方现网名为 X"；
- 并在 `unknowns[]` 里写明该别名**还能用多久未公布**。
- **同时要在治理清单里登记一条待办**：我们的目录是否应该继续售卖这个 ID。

理由：别名随时可能失效，而用户已经在按这个 ID 付费——这属于**产品风险**，不只是文档问题。

### 一个公开 ID 挂多条线路：只有"能力不一致"才是问题

**先分清两件事，不要混为一谈：**

| 情况 | 判定 | 处理 |
| --- | --- | --- |
| 同一能力，只是线路/价格不同 | **正常**，就是多档位 | 用分组与档位比例区分，**不拆 ID、不改基准价** |
| 线路背后的模型**能力不同**（支持的操作、参考图、分辨率、时长、上限不一样） | **问题** | 见下方处置 |

依据：仓库根 `AGENTS.md` 既有原则——"同量纲、仅价格区间不同 → 用分组，不拆 ID；
量纲不同才拆 ID"。**本契约不得与既有规则冲突；新增判据前必须先检索既有规则。**

能力不一致时的处置顺序（由轻到重）：

1. 有运行时分组契约时，由目录发布逐组的完整能力，支持该字段的调用方读取同一契约。公开文档解释字段，尚未纳入运行时分组契约的线路在文档中说明已核验的差异；
2. 只有当"必须给用户一个跨档位都成立的承诺"时，才拆 ID 或统一上游。

同时在档案里用 `routing` 字段逐条记录实际上游，`status` 用 `partial`。

### 判据纪律（新增判据前必须自检）

写在盘点、门禁、契约里的每一条"这是问题"，都必须同时给出三样东西，缺一不许落地：

1. **依据**：指向仓库既有规则，或指出为何既有规则不适用。
2. **反例**：什么情况下**不算**问题。写不出反例的判据，多半是拿"容易测量的"冒充"重要的"。
3. **受害者**：谁会因此受损。指不出受害者的，不是缺陷，只是现状。

反面教材（本契约自己的历史错误）：曾把"上游数量大于 1"直接判为缺陷，
而正确判据是"这些上游的能力是否一致"——数量容易从配置里算出来，
能力一致与否才是有意义的问题。用可测量的指标替代重要的指标，就是这类错误的来源。


## 三、按家族追加字段

### 3.1 `family: "image"`

```jsonc
{
  "reference_images": {
    "max": 16,                          // 数字或 null
    "min": 0,
    "formats": ["image/png", "image/webp", "image/jpeg"],
    "max_bytes_each": 52428800,          // 单张字节上限，null 表示未公布
    "source_claim": "单次最多 16 张参考图"
  },
  "output_sizes": ["1024x1024", "1536x1024"],
  "output_formats": ["png", "webp", "jpeg"],
  "prompt_max_chars": 32000
}
```

### 3.2 `family: "video"`

```jsonc
{
  "reference_images": { "max": 9, "min": 0, "formats": [...], "max_bytes_each": null },
  "reference_videos": { "max": 3, ... },
  "reference_audios": { "max": 3, ... },
  "durations_seconds": { "min": 4, "max": 30, "allowed": [4, 6, 8] },  // allowed 未公布则省略
  "resolutions": ["480p", "720p", "1080p"],
  "aspect_ratios": ["16:9", "9:16"],
  "audio_supported": true,
  "prompt_max_chars": null
}
```

### 3.3 `family: "audio"`

```jsonc
{
  "audio_kind": "tts",                  // tts | asr | music | voice-clone
  "input_formats": ["audio/wav", "audio/mpeg"],
  "max_input_bytes": null,
  "max_input_seconds": null,
  "output_formats": ["mp3"],
  "voices": null,                        // 数量或 null
  "languages": null
}
```

### 3.4 `family: "llm"`

```jsonc
{
  "context_window_tokens": 200000,
  "max_output_tokens": 64000,
  "input_modalities": ["text", "image"],
  "tool_calling": true,
  "streaming": true,
  "prompt_caching": false,
  "reasoning_efforts": { "low": "low", "medium": "medium", "high": "high" },
  "default_reasoning_effort": "medium"
}
```

`reasoning_efforts` maps the user-facing reasoning level to the upstream wire
value. Include only levels with an official source; omit the whole field when no
level is established. `default_reasoning_effort`, when present, MUST be one of
the keys in `reasoning_efforts`.

### 3.5 `family: "data"`（社交/抓取类接口，不是模型）

```jsonc
{
  "endpoints": [{ "name": "搜索帖子", "method": "GET", "path": "/..." }],
  "auth": "bearer",
  "rate_limit": null,
  "pagination": "cursor",
  "response_fields": ["id", "url", "..."]
}
```

### 3.6 推荐默认模型（`default_for`，所有 family 可选）

```jsonc
{
  "default_for": ["chat", "vision_chat"]
}
```

`default_for` 是网关的产品推荐，不是厂商能力声明，不需要 `sources[]`。取值只允许：

- `chat`、`vision_chat` → `family: "llm"`
- `text_to_image` → `family: "image"`
- `text_to_video` → `family: "video"`
- `text_to_music`、`speech_to_text` → `family: "audio"`

同一操作只能被一个模型推荐；`modelcontracts.Build` 与上线前的档案校验
都会拒绝未知键、重复推荐和 family 不匹配。推荐的 model 只有在
`/api/pricing` 同一响应中可见且 `enable_groups` 非空时才会出现在顶层
`defaults`；否则该键省略，绝不返回不可选的 model ID。

## 四、门禁约定（机器校验）

公开仓从根目录运行以下检查：

```sh
bun install --cwd web --frozen-lockfile
make model-contract-check
```

此命令检查生成物是否与档案一致、数字限制是否有来源，以及前端图像和视频表单与静态生成契约是否一致。
规格溯源检查拒绝声明了需溯源的数值限制、但没有 `sources[]` 的档案，并报告有来源但尚未逐项对应声明的限制。
公开 CI 的 Model capability contract 检查覆盖 Go 投影和数值溯源，表单一致性用例归入前端测试。

完整的档案校验、目录发布校验和在售契约完整性校验依赖实际目录基线，在上线前由部署维护者运行。
其中档案校验对照基线检查下列条件：

1. 目录里的**每个模型**都有对应档案文件；文件名 = `model_id`（`/` 替换为 `__`）。
2. 通用必填字段齐全且非空；`family` / `status` 取值在枚举内。
3. `status` 为 `documented` 或 `partial` 时，`sources[]` 至少一条，且每条含
   非空 `claim` / `url` / `quote`（长度 ≥ 8）/ `retrieved_at`。
   `url` 允许两种形式：`https://...`（厂商官方页），或 `repo:<仓内相对路径>`（我们自己的接口）。
   **`repo:` 形式的引文由门禁打开文件逐字核对**，对不上即失败。
4. 家族必填字段存在，类型正确。
5. 任何非 `null` 的数字型限制，必须能在 `sources[].claim` 中找到对应声明
   （通过 `source_claim` 或字段同名短语关联）。
6. `status` 为 `unverified` 时必须有 `unknowns[]` 且非空。

**这条门禁的意义**：以后任何人（包括 AI）想给某个模型加限制数字或宣称某能力，
先把档案写对；写不对就过不了门禁。这样"没做对"就有代价，不靠自觉。

## 五、档案的生命周期

- **新增模型**：先落档案 → 过门禁 → 才允许进目录。
- **厂商改规格**：更新档案的 `checked_at` 与 `sources`，重新生成发布契约，复核契约驱动的前端表单并同步双语公开文档。仅前端无法表达新契约结构、或旧版回退行为需要修复时，才涉及前端代码。
- **档案与实际执行不一致**：查明厂商规格、发布契约和选线各自的口径，修正错误的一侧。按原入口验证通过后才能宣称该能力可用。
- **定期复核**：季度巡检，只更新有变化的档案（模型规格很少变，所以成本很低）。

## 六、Contract v2: executable operation schema（schemaVersion 2.0）

> 面向客户端的契约从模型级能力扩展为**可执行操作档案**。v2 是增量扩展：历史档案无需迁移；
> 没有 `gateway` 块的模型仍按家族默认模板生成非可执行 fallback operations，并在 `unknowns[]`
> 记录未建立的输入/参数轴。

### 6.1 版本字段与兼容策略

- `schemaVersion`：字符串 `"2.0"`，同时出现在发布文档根与每个契约对象上。它只描述**发布契约**的
  解析版本；治理 spec 文件本身没有该字段也是合法的（视为 v1 作者输入）。
- `contractVersion`：每个模型独立计算的 16 位十六进制内容哈希，计算时排除该字段自身。任何模型级或
  操作级字段变化都会改变它，客户端与评审者可用它判断契约是否移动。
- `operation_ids`：字符串数组，是已发布 `operations[]` 的字符串视图。其内容统一按网关 operation
  词表规范化，因此旧客户端得到的列表与历史 spec 的原始字符串可能不完全相同；需要旧语义的客户端
  必须按 `schemaVersion` 分支或保留自己的映射。新客户端读对象化的 `operations`。
- `execution_status`：`live` / `stub` / `unavailable`。`live` 要求至少一个 `executable: true`
  的 operation；`stub` 表示已登记但没有已验证网关通路；`unavailable` 表示未在任何网关线路提供。
  非 live 必须在发布契约中带非空 `execution_reason`。spec 可显式写
  `execution: { "status": "stub" | "unavailable", "reason": "..." }`；省略时投影器会把没有可执行
  operation 的模型记为 `stub`。
- v2 只新增字段，不改变既有模型级字段的名字与含义（`reference_images`、`durations_seconds` 等全部
  保持 snake_case）。新增的 operation 子树使用 camelCase（`allowedMimes`、`maxSizeMb` 等），以匹配
  下游规范契约。
- 解析方遇到未知的 major `schemaVersion` 时应退化为模型级字段 + `operation_ids`，不得因为不能理解
  operation 子树而拒绝整个模型。

### 6.2 `gateway` 块（治理 spec 可选）

```jsonc
{
  "gateway": {
    "profile": "apimart_video",          // 当前：apimart_video | fal_h3_video | fal_h3_lip_sync | autodl_h3_video | openai_video
    "operations": [                       // 网关实际可执行的操作 ID，非空、无重复
      "text_to_video", "first_frame", "first_last_frame", "video_multi_ref"
    ],
    "mode_kinds": {                       // 可选，指定已声明操作的展示语义
      "video_multi_ref": "multimodal_reference"
    },
    "source_claim": "一句话说明该 profile 与操作集合的代码依据"
  }
}
```

- `gateway` 是**网关侧已核验**的执行档案：只有本仓请求校验代码真的接受的操作才能写进来。厂商文档
  列出但本网关没有实现通路的能力，只能留在 spec 的 `operations`（厂商能力）里，不能写进
  `gateway.operations`。
- `profile` 是投影模板名，不是自由文本。当前只允许 `apimart_video`、`fal_h3_video`、
  `fal_h3_lip_sync`、`autodl_h3_video`（AutoDL task plugin，channel type 65）与
  `openai_video`（task plugin host protocol，`POST /v1/videos` + `GET /v1/videos/:task_id`，
  见 `pkg/jsplugin/routing.go`）；新增模板必须在 `modelcontracts` 内注册并同步扩展门禁白名单。
- `gateway.operations` 必须与本仓实际执行代码一致（`relay/channel/task/apimart/video_contract.go`
  的 `operationAllowed`、`relay/channel/task/fal/media_inputs.go` 的实际分支、
  `plugins/tasks/autodl` 的 shapes 映射，或 `pkg/jsplugin/routing.go` 的 `openai_video`
  host protocol），并在 `sources[]` 中用 `repo:` 引文留证；`source_claim` 只是摘要，
  不能替代 `sources[]`。
- `gateway.mode_kinds` 的键必须出现在 `gateway.operations` 中。视频 `video_multi_ref` 使用
  `image_reference` 表示仅使用图片作为参考，使用 `multimodal_reference` 表示支持多个素材类型。
  其他视频操作以自身 ID 表达语义。生成契约将该值发布为 `operations[].modeKind`。
- 新公开 ID 可在 `apimart_video` 下用 `gateway.request_model` 复用已登记的原生请求解码器。
  当前可复用 `seedance-2-5` 解码器，经运行时渠道规则登记后才可发布到付费目录。该字段是档案内部的解码器选择，
  生成契约只公开 `gateway_protocol: "video_generations"`。已有档案沿用原发布字段。
- 操作 ID 使用网关统一词表（视频类 `text_to_video`、`first_frame`、`end_frame`、
  `first_last_frame`、`video_multi_ref`、`video_edit`、`video_extend`、`document_to_video`、
  `webpage_to_video`；图像类 `text_to_image`、`image_to_image`、`multi_reference`；文本类 `chat`；
  音频类 `speech_to_text`、`text_to_speech`、`voice_clone`、`text_to_music` 等）。spec 的
  `operations` 与 `gateway.operations` 允许不一致——前者是厂商口径，后者是本网关实际可执行的较窄
  集合。

### 6.3 `parameters` 块（治理 spec 可选）

```jsonc
{
  "parameters": [
    {
      "name": "aspect_ratio",
      "type": "enum",                    // enum | integer | number | boolean | string
      "enum": ["auto", "1:1", "16:9"],    // 仅 type=enum；非空、元素类型一致
      "default": "auto",
      "applies_to": ["text_to_image", "image_to_image"],
      "limit_source": { "kind": "policy_conservative", "note": "前端既有参数表；上游未公布枚举" }
    },
    { "name": "n", "type": "integer", "minimum": 1, "maximum": 1, "default": 1, "fixed": 1 }
  ]
}
```

| 字段 | 类型 | 语义 |
| --- | --- | --- |
| `name` | string | 参数名，必填非空 |
| `type` | string | `enum` / `integer` / `number` / `boolean` / `string` 之一 |
| `enum` | array | 仅 `type=enum` 时出现；非空，元素类型一致 |
| `minimum` / `maximum` | number | 有限数字；两者同时存在时必须 `minimum <= maximum` |
| `step` | number | 可选步进，正数 |
| `default` | 与 `type` 匹配 | 默认值；enum 必须取 `enum` 中的值 |
| `fixed` | 与 `type` 匹配 | 锁定值：UI 不可改、请求必须为该值；enum 必须取 `enum` 中的值 |
| `applies_to` | string[] | 适用的操作 ID；缺省表示适用于该模型全部可执行操作 |
| `limit_source` | object | `{ kind, note }`，kind 见下表 |

`limit_source.kind` 语义：

| kind | 含义 | 溯源要求 |
| --- | --- | --- |
| `official_docs` | 厂商官方页面或本仓官方接口文档 | 数字或字段名必须能在 `sources[].claim` / `quote` 中找到 |
| `measured` | 本线路实测值 | 必须写明测点与证据 |
| `policy_conservative` | 平台侧保守策略、前端既有参数表或本服务 service_notes，上游未公布 | 允许没有官方 claim；`note` 必须写清来源与保守理由 |

### 6.4 发布契约中的 `operations` 对象

```jsonc
{
  "id": "first_frame",
  "modeKind": "first_frame",
  "executable": true,
  "inputs": [
    { "slot": "prompt", "type": "text", "required": true },
    { "slot": "first_frame", "type": "image", "role": "first_frame", "required": true, "min": 1, "max": 1 }
  ],
  "parameters": [
    { "name": "resolution", "type": "enum", "enum": ["480p", "720p", "1080p"], "default": "720p" },
    { "name": "aspect_ratio", "type": "enum", "enum": ["adaptive"], "fixed": "adaptive" }
  ],
  "output": { "type": "video" }
}
```

- `id` 必填且非空；同一契约内不得重复。
- `modeKind` 是可选的本地化键。同一模式的数量限制变化时继续使用该键，数量取自 `inputs`。
  字段缺省时沿用 `id` 的展示映射，提交请求始终使用 `id`。
- `executable`：`true` 表示本网关存在可执行通路（来自 `gateway` 模板核验）；`false` 表示只从厂商
  文档列出、未经本网关验证，`inputs` / `parameters` 可为空，但对应缺口必须写进契约 `unknowns[]`。
- `inputs[]`：`slot`（输入槽名）、`type`（text/image/video/audio/file/url 等）、`role`（可选，如
  `first_frame` / `last_frame`）、`required`、`min` / `max`（数量约束）、`allowedMimes`、
  `maxSizeMb`、`minDurationSec` / `maxDurationSec`。未确立的轴省略，不得用 0 或默认值代替。
- `firstItemRole`：可选字段。值为 `first_frame` 时，有序参考图列表的第一项默认作为起始画面，其余作为参考。首帧独立输入槽仍使用 `role: first_frame`。
- `gateway_protocol` 是可选的模型默认协议。逐组有效操作也可提供同名字段来指定该操作变体的协议，见 §6.9。
- `parameters[]`：从 6.3 的 `parameters` 与 profile 模板展开；`enum` / `minimum` / `maximum` /
  `step` / `default` / `fixed` / `limitSource` 含义与 6.3 相同（发布契约中使用 `limitSource`）。
- `output`：必填对象，至少含 `type`（image/video/audio/text/data 等）；`formats`、`sizes` 可选。

### 6.5 `unknowns` 与 `null` 规则

- 发布契约的 `unknowns[]` 是**稳定的字段路径**，不是散文：使用点号拼接，例如
  `reference_images.max_bytes_each`、`durations_seconds.max`、
  `operations.first_frame.parameters.duration`。治理 spec 里原有 `unknowns[]` 仍是作者说明文字，
  **不发布**到契约；契约的路径由投影器按“家族必填轴为 null / 缺省”生成。
- 字段为 `null` 表示“厂商未公布 / 未确立”，不是 0；缺省同样表示未确立，调用方不得替换成默认值。
- 厂商明确说“无限制”时：值为 `null`，在 `notes` 写明官方口径，且**不**把该字段列入 `unknowns[]`
  （沿用第二节 `null` 的两种含义）。
- `executable: false` 的操作缺少的输入 / 参数轴，必须在 `unknowns[]` 以
  `operations.<operationId>...` 路径记录。

### 6.6 门禁

依赖目录基线的档案校验也检查以下可选块：

- `gateway.profile` 只能是已注册模板；`gateway.operations` 非空且无重复。
- `parameters` 必须是数组；`name` 非空，`type` 合法，`enum` 非空且类型一致，
  `minimum` / `maximum` 为有限数字且有序，`default` / `fixed` 与 `type` 匹配，
  `limit_source.kind` 在允许枚举内。

`scripts/ops/check-spec-traceability.mjs` 把 `parameters[].minimum` / `maximum` 纳入数值溯源；
`limit_source.kind = policy_conservative` 的参数允许没有官方 claim，其余非 null 数值必须能在
`sources[].claim` / `quote` 中找到数字或字段名。

契约生成器校验 `gateway.mode_kinds` 的操作 ID、语义键和参考素材类型是否匹配。

上线前的在售契约完整性校验覆盖实际目录：所有 `status=1` 的在售模型必须
`execution_status: live` 且至少一个 `executable: true` operation；executable operation 不得在
`unknowns[]` 中留下 `operations.<id>.inputs` / `operations.<id>.parameters` 缺口。没有 gateway
profile 的历史档案可以作为 `stub` 保留，但不得继续在售。

### 6.7 新增模型的 Definition of Done

1. 先写/更新 `docs/model-governance/specs/<model_id>.json`；不得先改前端或 Go 适配器来“补”模型。
2. 选择已注册 profile；新 profile 必须同时补 `modelcontracts/operations.go` 的模板与 adapter parity 测试。
3. 运行 `go run ./modelcontracts/gen`，确认 `contractVersion` 变化且 `execution_status` 正确。
4. 运行 `make model-contract-check`。在目标测试环境从目录入口验证展示、提交和结果，异步任务使用查询入口取得最终结果。按下文记录本次实测。上线前另由部署维护者运行依赖目录基线的检查。
5. 公开目录/模型页变更同步双语公开文档、changelog 与模型页验收。涉及前端代码时按 [Surfaces](../../AGENTS.md#surfaces-fork) 的 HTML 演示确认规则执行。

没有 `gateway` / `parameters` 的历史档案不受 v2 结构本身影响，但只有 `stub`/`unavailable` 状态可以保留；
`status=1` 的在售模型必须完成上述 Definition of Done。

#### 发布前独立复核

新增模型或分组，或更新现有模型或分组的对外能力、参数或上游接口时，适用以下流程。

1. **实施 Agent 实测并留证。** 复核前选一个能体现本次改动的样例，通过网关完成一次真实生成。
   记录实测版本与环境、涉及的模型和分组、实际发给上游的脱敏请求和结果，以及对应上游接口的官方文档链接和原文。
   请求样例保留待核对的字段和数据格式，证据按[公开仓边界](../../AGENTS.md#public-boundary-hard-gate)保存在获准复核者可访问的位置。
2. **另一位 Agent 只核对材料。** 复用这次生成的记录，检查每个受影响的模型或分组应填的素材格式、数量和参数是否已经登记，
   确认样例应传的参数确实送到了上游。对照实际使用的上游接口文档，核对字段名、结构、类型、格式、必填条件、单位、允许值、范围和数量。
   样例只证明本次用到的组合，其余声明也要有对应依据。已有的 PR 独立审查可以一并完成这项复核。
3. **在原工单或 PR 记录结论。** 记录可公开的证据摘要、已核实项和待核实项。上游文档缺项、与实际请求不符，
   或实测与对外声明不一致时，写明具体参数和差异，交实施 Agent 补证或修正。按 §6.5 记录仍未核实的规格。

### 6.8 运行时发布门禁（Runtime publication gate）

`/api/pricing` 只发布同时满足以下条件的模型：

1. 模型仍有启用渠道可路由（Served）；
2. 契约 `execution_status` 为 `live`；
3. 契约至少有一个 `executable: true` 的 operation。
4. 新档案声明 `gateway_protocol: video_generations` 时，该公开模型已登记到运行时视频分组规则。

不满足 2/3 的模型属于 **routable but unlisted**：仍可按模型 id 显式调用，但不出现在
公开定价/目录响应中，也不得视为在售。`SelfUseModeEnabled` 为自建/BYO 部署保留旧行为；
`OMNIMUX_MODEL_CATALOG_ALLOW_UNCONTRACTED=1` 是应急开关。若契约载荷本身不可读，
网关保持发布现有 Served 目录，不得把 `/api/pricing` 清空。

#### 定价目录的模型分组

`GET /api/pricing` 的每个模型行返回以下展示字段。已有 `enable_groups` 的成员和顺序照常返回。

| 字段 | 含义 |
| --- | --- |
| `group_order` | 模型级在售分组顺序。与该行 `enable_groups` 集合相同，每组恰好出现一次。登记有效时保持跨请求和重启稳定。下线或未知的配置项不会加入目录。 |
| `default_group` | 该模型登记的默认组。组名须同时属于该行 `enable_groups` 和调用者的 `usable_group`，否则返回空字符串。模型全局只售一组时，顺序仅含该组，该组对调用者可用且登记有效时也就是默认组。 |

`auto_groups` 是当前账号用于自动路由的有序候选。`group_order` 用于模型的线路展示。旧客户端可继续读取原字段。顶层 `pricing_version` 标识接口字段版本，运营编辑展示配置时保持该字段版本。

有效顺序缺失时，`group_order` 原样沿用该行 `enable_groups` 的数组顺序。只在默认组缺配置时，依次取 `user.Group`、`default`、`group_order` 中第一个可用组。每一步都检查模型启用组和调用者可用组，找不到则返回空字符串。配置读取失败按缺配置处理。已登记的默认组因权限或目录变更变得不可用时返回空字符串。

**维护与发布检查**

超管通过 `GET /api/option/?key=ModelPricingGroups` 读取 `data[0].value`。该值是含 `version` 与 `models` 的 JSON 字符串，`models` 以精确的目录模型 ID 为键。未登记的文档版本为 `0`。下面的 `PUT /api/option/` 示例适用于空配置的隔离测试目录。

```json
{
  "key": "ModelPricingGroups",
  "value": "{\"version\":0,\"models\":{\"gpt-image-2.5\":{\"default_group\":\"fixture-lime\",\"group_order\":[\"fixture-lime\"]}}}"
}
```

保存前读取当前文档，使用精确的 `ModelPricingGroups` 键，提交完整 `models` 和读到的 `version`。保存会校验当前目录中的模型和分组并推进版本，遇到 HTTP `409` 时重新读取后再编辑。每次 `GET /api/pricing` 在现有定价缓存之外按主键读取一个 option。共享同一主库的实例在保存后的下一次成功读取生效，无配置缓存同步等待。

下方命令以只读方式检查当前完整发布目录，包含普通调用者不可见的模型。检查直接从数据库读取目录及适用的渠道设置。配置或目录读取失败时返回 HTTP `503`。命令同时校验 HTTP `200` 和正文中的唯一 JSON 对象。存在缺项或无法检查时以非零状态退出，缺失或失配的模型写入标准错误。全部登记时应得到 `[]`。

```sh
pricing_group_check_response=$(
  curl --fail --silent --show-error \
    --header "Authorization: Bearer ${GATEWAY_ROOT_ACCESS_TOKEN:?}" \
    --write-out '\n%{http_code}\n' \
    "${GATEWAY_URL:?}/api/option/?key=ModelPricingGroups"
) && printf '%s\n' "$pricing_group_check_response" |
  jq -s -e '
    # The second value is the HTTP status appended by curl.
    if length != 2 or .[1] != 200 or (.[0] | type) != "object" then
      error("expected an HTTP 200 response with one JSON object")
    elif .[0].success != true or (.[0].pricing_group_config_issues | type) != "array" then
      error("cannot check pricing group configuration")
    else
      .[0].pricing_group_config_issues | if length == 0 then . else halt_error(1) end
    end'
```

同一超管响应提供本实例自启动以来的 `pricing_group_fallback_rows` 和 `pricing_group_read_failures`。前者按触发防御行为的模型响应行计数，后者按读取或解码配置失败的请求计数。

### 6.9 视频分组的有效能力

`/api/pricing` 的模型行和 `/api/model_contract/:model` 的 `data` 可在 `contract` 同级返回
`effective_video`。其 `groups` 把当前账号可选的逻辑分组映射到完整操作变体，`version` 随该视角下
的能力内容变化。模式、素材槽和参数需要同时匹配一个变体。相同操作 ID 可以有多个变体，参数上的
`required: true` 表示调用方须显式传值。变体默认沿用模型级契约的 `modeKind` 和素材角色，
已核验的 profile 可以将 `modeKind` 收窄到实际接受的素材类型。

缺省 `effective_video` 时保留模型级 `contract` 和原调用方式。空 `groups` 表示当前视角没有可发布的分组。
组内空数组表示该组目前没有经过核验的变体。`effective_video.groups` 是目录中该账号逐组的能力视图。
请求使用实际提交令牌的访问权限。显式选择具体组，例如 `model@group`，会把提交和重试限制在该组。
已核验的候选都不支持当前组合时返回 `400 video_capability_mismatch`。当前没有可用的已核验候选时返回 `503`。

调用方优先读取操作变体的 `gateway_protocol`，缺省时继承 `contract.gateway_protocol`。试点旧版的两个字段都可能缺省，
此时沿用旧客户端已知的提交方式。协议确定创建、查询入口及请求格式。

| 协议 | 创建 | 查询 |
| --- | --- | --- |
| `openai_video` | `POST /v1/videos` | `GET /v1/videos/:task_id` |
| `video_generations` | `POST /v1/video/generations` | `GET /v1/video/generations/:task_id` |

原生协议可以在创建请求中携带 `X-OmniMux-Video-Protocol: video_generations`。
当模型默认协议是 `openai_video` 且选择原生变体时，在原生创建入口携带该请求头。旧客户端访问原生创建入口时，
原有的 OpenAI Video 模型默认解码行为继续有效。重试使用初次请求的协议和能力条件。查询使用创建返回的任务 ID 及对应协议，
并用原任务记录获取结果。

网关启动时可从 `OMNIMUX_VIDEO_CAPABILITIES_FILE` 装载分组规则。配置的 `models` 把已有可执行视频档案并使用已核验
解码器的公开 ID 映射到各自的渠道规则数组。每条规则包含 `channel_id`、
`channel_type`、`profile`、`upstream_model`、`base_url`、`evidence` 与 `operations`。
变体引用既有操作 `id`，可用 `inputs` 和 `parameters` 限定该变体接受的槽位与参数。
规则需要与当前通道类型、上游模型映射、地址和插件实现一致。渠道身份、地址和证据留在规则配置中。
原生规则可指定 `request_model`，用于在模型默认协议不同的情况下选择已核验的解码器。
同一公开模型的原生规则共享一个解码器。`openai_video` profile 使用网关内置的 Muse 有序参考图解码器，
新公开 ID 由渠道模型映射到该解码器的既有目标。Muse 解码器补齐 5 秒默认时长，变体声明 `parameters` 时需包含
`duration`，范围不含 5 秒时调用方须显式传值。新增原生公开 ID 的变体要求调用方显式填写正时长。

`seedance25_openai_video` profile 仅用于 `seedance-2-5` 的 OpenAI 类型渠道，由工厂 Sora 插件解码并按渠道映射提交。
对外协议为 `openai_video`。`video_multi_ref` 变体发布 `modeKind: "image_reference"`，用 `reference_images` 槽承载一张无首尾帧角色的图片 URL。
调用方使用 `images` 或 `image_urls`，二选一。明确传入 `duration`（也接受 `seconds`）和 `aspect_ratio: "9:16"`。
同组只有一个已核验的经济档分辨率时，目录和网关采用该值作为 `resolution` 的默认值。
同组有多个经济档分辨率时，各经济档变体要求调用方显式传入 `resolution`，目录省略默认值。
原生创建入口的无协议头请求在选定只有该 profile 的具体组时也走这个解码器。混合协议组和 `auto` 在该入口沿用原生默认值。
首帧语义、供应侧实际图片上限和成片规格仍待实测。规则的单图上限只表示网关承诺受理的请求形状。
五组[本地规则样例](../../router/testdata/issue-74/five-group-rules.json)使用合成渠道和回环 mock，可用
`go test ./router -run '^TestFiveVideoGroups$' -count=1` 验证目录投影、外发请求和原任务回收。
用 `OMNIMUX_TEST_GATEWAY_BINARY` 指定从仓库根目录构建的网关二进制，可在同一用例中改用独立网关进程和临时 SQLite。
设置 `OMNIMUX_TEST_FIVE_GROUP_CATALOG` 为输出路径，可导出实测目录供客户端离线复验。

从 #68 双模型试点扩展到复用已核验解码器的验收记录见 [#74](https://github.com/omnimux-ai/omnimux-gateway/issues/74)。
调用方可使用由两个目录入口实际生成的[同协议样例](../../router/testdata/issue-74/catalog.json)
和[混合协议样例](../../router/testdata/issue-74/mixed-catalog.json)。样例仅包含各自的合成模型及用于选组的响应字段。
普通测试核对保存的样例，重新生成时运行
`OMNIMUX_UPDATE_GROUPED_VIDEO_FIXTURES=1 go test ./router -run '^TestRegisteredVideoProtocolCatalogAndRoutes$' -count=1`。

## Field: `basis`

Distinguishes a spec that records the **vendor model's published capabilities**
from one that records **our own line's restrictions**. A reverse-engineered
channel can accept materially less than the vendor documents — a fixed length, a
single resolution, and a lower reference-material ceiling — and nothing else in
the spec says which of the two a number came from.

| Value | Meaning |
| --- | --- |
| absent | Not classified yet. Numbers are the vendor model's unless a `source_claim` says otherwise. |
| `vendor_official` | The numbers are the vendor's published capabilities. |
| `line_restricted` | The numbers are our line's restrictions, which are narrower than the vendor's. |

Classification proceeds model by model; an absent value means "not yet
classified", never "no restriction". The field is projected into the published
contract (`contracts.json` → `basis`), so a client can tell the two apart without
reading this document.
