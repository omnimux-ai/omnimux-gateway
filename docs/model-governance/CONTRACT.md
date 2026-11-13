# 模型档案契约（Model Spec Contract）

> 目的：把"每个模型到底支持什么、限制是什么、依据是什么"变成**落盘的、可复用的、可被门禁校验的**事实，
> 而不是靠记忆、上下文或临时搜索。模型规格很少变，所以一份档案可以被反复使用。

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

1. 先在模型页与公开文档**明示各档差异**（哪个档支持什么），让用户的预期与事实一致；
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

同一操作只能被一个模型推荐；`modelcontracts.Build` 与 `check-model-specs.mjs`
都会拒绝未知键、重复推荐和 family 不匹配。推荐的 model 只有在
`/api/pricing` 同一响应中可见且 `enable_groups` 非空时才会出现在顶层
`defaults`；否则该键省略，绝不返回不可选的 model ID。

## 四、门禁约定（机器校验）

`scripts/ops/check-model-specs.mjs` 对每个档案做确定性校验，任一条不满足即失败：

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
- **厂商改规格**：更新档案的 `checked_at` 与 `sources`，同一变更集里同步前端限制表与公开文档。
- **档案与线上不一致**：以档案为准，视为线上缺陷。
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
    "profile": "apimart_video",          // 当前：apimart_video | fal_h3_video | fal_h3_lip_sync | autodl_h3_video
    "operations": [                       // 网关实际可执行的操作 ID，非空、无重复
      "text_to_video", "first_frame", "first_last_frame", "video_multi_ref"
    ],
    "source_claim": "一句话说明该 profile 与操作集合的代码依据"
  }
}
```

- `gateway` 是**网关侧已核验**的执行档案：只有本仓请求校验代码真的接受的操作才能写进来。厂商文档
  列出但本网关没有实现通路的能力，只能留在 spec 的 `operations`（厂商能力）里，不能写进
  `gateway.operations`。
- `profile` 是投影模板名，不是自由文本。当前只允许 `apimart_video`、`fal_h3_video`、
  `fal_h3_lip_sync` 与 `autodl_h3_video`（AutoDL task plugin，channel type 65）；新增模板必须在
  `modelcontracts` 内注册并同步扩展门禁白名单。
- `gateway.operations` 必须与本仓实际执行代码一致（`relay/channel/task/apimart/video_contract.go`
  的 `operationAllowed`、`relay/channel/task/fal/media_inputs.go` 的实际分支，或
  `plugins/tasks/autodl` 的 shapes 映射），并在 `sources[]` 中用 `repo:` 引文留证；
  `source_claim` 只是摘要，不能替代 `sources[]`。
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
- `executable`：`true` 表示本网关存在可执行通路（来自 `gateway` 模板核验）；`false` 表示只从厂商
  文档列出、未经本网关验证，`inputs` / `parameters` 可为空，但对应缺口必须写进契约 `unknowns[]`。
- `inputs[]`：`slot`（输入槽名）、`type`（text/image/video/audio/file/url 等）、`role`（可选，如
  `first_frame` / `last_frame`）、`required`、`min` / `max`（数量约束）、`allowedMimes`、
  `maxSizeMb`、`minDurationSec` / `maxDurationSec`。未确立的轴省略，不得用 0 或默认值代替。
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

`scripts/ops/check-model-specs.mjs` 在原有规则之外，对**出现即校验**的可选块执行：

- `gateway.profile` 只能是已注册模板；`gateway.operations` 非空且无重复。
- `parameters` 必须是数组；`name` 非空，`type` 合法，`enum` 非空且类型一致，
  `minimum` / `maximum` 为有限数字且有序，`default` / `fixed` 与 `type` 匹配，
  `limit_source.kind` 在允许枚举内。

`scripts/ops/check-spec-traceability.mjs` 把 `parameters[].minimum` / `maximum` 纳入数值溯源；
`limit_source.kind = policy_conservative` 的参数允许没有官方 claim，其余非 null 数值必须能在
`sources[].claim` / `quote` 中找到数字或字段名。

`scripts/ops/check-model-contract-completeness.mjs` 是上架门禁：所有 `status=1` 的在售模型必须
`execution_status: live` 且至少一个 `executable: true` operation；executable operation 不得在
`unknowns[]` 中留下 `operations.<id>.inputs` / `operations.<id>.parameters` 缺口。没有 gateway
profile 的历史档案可以作为 `stub` 保留，但不得继续在售。

### 6.7 新增模型的 Definition of Done

1. 先写/更新 `docs/model-governance/specs/<model_id>.json`；不得先改前端或 Go 适配器来“补”模型。
2. 选择已注册 profile；新 profile 必须同时补 `modelcontracts/operations.go` 的模板与 adapter parity 测试。
3. 运行 `go run ./modelcontracts/gen`，确认 `contractVersion` 变化且 `execution_status` 正确。
4. 运行 `make model-contract-check`；所有门禁和前端 contract parity 必须通过。
5. 公开目录/模型页变更同步 public docs、changelog 与模型页验收；涉及前端时按 §Surfaces 的 HTML 演示确认规则执行。

没有 `gateway` / `parameters` 的历史档案不受 v2 结构本身影响，但只有 `stub`/`unavailable` 状态可以保留；
`status=1` 的在售模型必须完成上述 Definition of Done。

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
