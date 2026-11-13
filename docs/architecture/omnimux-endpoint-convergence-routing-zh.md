# 端点收敛与路由设计：同一模型 ID + 分组/请求形态选端点

| 字段 | 内容 |
| --- | --- |
| 文档版本 | v1.0 |
| 日期 | 2026-09-12 |
| 目标 | 把「同一能力的多个上游端点」收敛到**一个公开模型 ID**，用**分组 + 请求形态**决定实际调用哪个端点 |
| 样本场景 | AutoDL.Art ComfyUI 工作流（14 个上游端点，见 [调查报告](../ops/autodl-comfyui-api-survey-zh.md)） |
| 结论 | **本仓库已有全部原语，无需新增计费/路由机制**；推荐用 **Task Plugin（JS）** 承载端点选择，**零 Core-Change** |

---

## 1. 结论（TL;DR）

1. **分组不是抽象的折扣标签，它就是端点选择器**：`分组 → 渠道 → 渠道 model_mapping → 上游端点`。每个分组绑定各自的渠道，同一公开 ID 在每个渠道上映射到不同上游端点，`GroupRatio` 决定该端点的卖价倍率。
2. **请求形态是第二级选择器**，落在适配器/插件内部：解析请求体里出现了哪些能力槽位（`first_frame+last_frame` / `ref_audio` / `ref_image` / 仅 `prompt`），决定同一个渠道内使用哪个上游端点。
3. **插件里能看到分组选中的端点**：`DriverContext.upstreamModel` 来自渠道 `model_mapping`，所以「分组的选择结果」会传进 JS，插件据此再叠加请求形态做最终判定。
4. **价格有三个原生通道**，不需要新账本：`ModelPrice(公开 ID) × GroupRatio(分组) × usage facts(时长/分辨率)`；此外若映射到的上游名自带 tiered 表达式，会用上游名的表达式。
5. **AutoDL 可以零 Core-Change 接入**：`Task Plugin` 渠道类型 + 单文件 ES Module 插件（`buildSubmitRequest` 里拼 `/comfyui_workflow/{workflow_id}`）。`relay/`、`constant/` 都不用改。
6. **两个硬约束必须先解决**：① `indextts2` 是按次、其余 13 个按秒 → 必须拆成不同公开 ID（`BILLING_DIMENSION_CONFLICT` P0 门禁）；② 1080p 在 3 个端点上存在 **0.09 与 0.10 两个单价**、峰值/空闲折扣率**逐档不同**（0.5/0.67/0.56/0.6），单一时间窗倍率无法同时还原，分时价不能靠一个时间乘子实现。

---

## 2. 现有原语（含证据）

### 层 1｜分组解析（无状态、每请求）

| 事实 | 证据 |
| --- | --- |
| 三个来源，实际优先级 **Header > `model@group` 后缀 > 请求体 `group`**（代码按 body → 后缀 → Header 顺序覆盖） | `middleware/distributor.go:47-70` |
| 支持 `X-Omnimux-Group` 与 `New-Api-Group` 两个头 | `middleware/distributor.go:66-70` |
| 门禁：分组必须在用户的可用分组内，且非 `auto` 时必须在 `GroupRatio` 表中存在，否则 403 | `middleware/distributor.go:73-93` |
| 通过后写入 `ContextKeyUsingGroup` / `ContextKeyTokenGroup`，供后续选渠道与计价 | `middleware/distributor.go:95-99` |
| 选渠道：`CacheGetRandomSatisfiedChannel({ModelName, TokenGroup, RequestPath, Retry})` | `middleware/distributor.go:198-218` |
| 渠道必须落在该 `(group, model)` 能力矩阵内（内存 `group2model2channels`） | `model/channel_satisfy.go:10-30` |
| `auto` 是有序分组列表（token 可持有快照），逐个尝试 | `service/group.go:94-107` |

### 层 2｜渠道 → 上游端点映射（配置）

| 事实 | 证据 |
| --- | --- |
| 渠道有 `model_mapping` 字段（JSON：`公开ID → 上游ID`） | `model/channel.go:42`、`model/channel.go:532-536` |
| 分发阶段把渠道 mapping 放进请求上下文 | `middleware/distributor.go:711` |
| 映射是**链式**的，带环检测；命中后 `IsModelMapped=true`、`UpstreamModelName=链尾`，并 `request.SetModelName(上游名)` | `relay/helper/model_mapped.go:14-67` |
| 任务链路时序：先做映射，再 `ValidateRequestAndSetAction` | `relay/relay_task.go:222-245` |

> 推论：**同一公开 ID，在不同分组（=不同渠道）下可以指向完全不同的上游端点**。这是「分组路由端点」的全部机制，纯配置。

### 层 3｜渠道内按请求形态二次分发

| 先例 | 机制 | 证据 |
| --- | --- | --- |
| GoEasy `gpt-image-2.5` | 一个公开 ID，按 `size`(2k/4k) 与 `quality`(hd/pro/high) 把上游名改写成 `image2.5-pro-async`，否则 `gpt-image-2.5-async` | `relay/channel/task/goeasy/adaptor.go:151-176`、别名归一 `:216-236` |
| GXGenAI / RunningHub | mapping 值是**数字** → `POST /openapi/v2/ai-app`；含 `/` → `POST /openapi/v2/{endpoint}` | `relay/channel/task/gxgenai/adaptor.go:27`、`:108-113` |
| 路径型端点可行性 | `DoTaskApiRequest` 中 `BuildRequestURL` 在 `BuildRequestBody` 之后调用，适配器可在校验/建体阶段 `c.Set(...)` 暂存解析结果再读 | `relay/channel/api_request.go:583-606`；`relay/relay_task.go:339`；goeasy 的 `c.Set("task_quality", ...)` 即此用法 |

### 层 4｜定价（宿主所有，插件不得算价）

| 事实 | 证据 |
| --- | --- |
| tiered 模式优先取**公开 ID** 的表达式；若 `IsModelMapped` 且**上游名**有表达式，则改用上游名的表达式 | `relay/relay_task.go:247-264` |
| 非 tiered：按 **`info.OriginModelName`（公开 ID）** 查 `ModelPrice`/`ModelRatio`，再乘 `GroupRatio` | `relay/helper/price.go:195-226` |
| 适配器/插件的 usage facts（秒、分辨率等）以 `OtherRatios` 原生乘法叠加（饱和转换） | `relay/relay_task.go:302-328` |
| 提交后按上游真实参数调整、终态结算 | `relay/relay_task.go:368-379`、`service/task_billing.go` |
| 用量 hook 只能返回事实（`second`/`count`/`token`/`credit`），不得算价 | `docs/plugin-api/README.md:140-142` |

### 层 5｜Task Plugin（可替代新增 Go 渠道类型）

| 事实 | 证据 |
| --- | --- |
| 接入第三方平台 = **创建 `Task Plugin` 类型渠道 + 选插件 key + 配 base URL + 配模型**，无需新 Go 渠道类型 | `docs/plugin-api/README.md:120-125` |
| 插件导出 `meta` / `buildSubmitRequest` / `parseSubmitResponse` / `parseTaskResult`；`per_task` 加 `buildQueryRequest` | `docs/plugin-api/README.md:9-11` |
| `NativeRoute.models` 由宿主在**任何 JS 钩子之前**用请求体顶层 `model` 匹配，「空 = 不限制」，必须是 `meta.models` 子集 | `pkg/jsplugin/routing.go:24-35` |
| `buildSubmitRequest` 返回自由 `RequestDescriptor{url, method, headers, body, ...}` → URL 与体内任一字段都由 JS 决定；可 `rewriteModel` | `docs/plugin-api/v1.d.ts:31` |
| `DriverContext` 暴露 `requestBody / model / upstreamModel / baseUrl / apiKey / files / publicTaskId / originTasks` | `docs/plugin-api/v1.d.ts:27` |
| `meta.usageSchema` 单位枚举含 `second` 与 `count`，与 AutoDL 两种量纲对齐 | `docs/plugin-api/v1.d.ts:23`、`:25` |
| 同一模型可由多个 provider 实现，候选顺序确定 | `pkg/jsplugin/routing.go:466-480` |

---

## 3. 选择器优先级（设计契约）

```text
① 分组（谁的成本池/档位）
   Header X-Omnimux-Group  >  model@group  >  body.group  >  令牌默认分组(auto)
        └─→ 选中渠道 ─→ channel.model_mapping[公开ID] ─→ ctx.upstreamModel（档位 pin）
② 请求形态（同一档位内用哪个端点）
   插件 buildSubmitRequest 读 ctx.requestBody 的能力槽位 + 能力矩阵过滤
        └─→ 得到具体 workflow_id
③ 参数只影响“价格”，不影响端点（除非端点不支持该档位 → 能力裁剪）
   resolution / duration ─→ usage facts ─→ OtherRatios
```

**关键点**：`ctx.upstreamModel` 是分组选择结果进入 JS 的唯一通道 —— 这就是「用分组路由端点」与「用请求格式路由端点」的接合面。若某分组不配 `model_mapping`，`upstreamModel == model`，插件走内置默认矩阵。

---

## 4. AutoDL 收敛设计

### 4.1 端点能力矩阵（来自本地数据表，实测）

| workflow（上游 ID） | 形态 | 时长上限 | 分辨率档 | 峰值/空闲（￥/秒） |
| --- | --- | --- | --- | --- |
| `minimax_h3_lightx2v_no_pic` | 文生视频 | 15 | 480p/768p | 0.02/0.01 · 0.03/0.02 |
| `minimax_h3_b99_001` | 文生视频 | 15 | 736p | 0.03/0.02 |
| `minimax_h3_lightx2v_v5` | 多图参考 | 10 | 480p/768p/**1080p** | …· 0.09/0.05 |
| `minimax_h3_lightx2v_v5_15s` | 多图参考 | 15 | 480p/768p | … |
| `minimax_h3_b99_003_12s` | 多图参考 | **12** | 736p | 0.03/0.02 |
| `minimax_h3_lightx2v` | 首尾帧 | 15 | 480p/768p | … |
| `minimax_h3_b99_002` | 首尾帧 | 15 | 736p | 0.03/0.02 |
| `minimax_h3_zm_u24` | 多图多音频（升级画质） | 15 | 480p/768p | … |
| `minimax_h3_zm_u08` | 多图多音频（高速版） | 15 | 480p/768p | … |
| `minimax_h3_image_audio_to_video_v2` | 多图多音频 | 10 | +**1080p 0.10/0.06** | |
| `minimax_h3_image_audio_to_video_v2_15s` | 多图多音频 | 15 | 480p/768p | … |
| `minimax_h3_image_audio_to_video` | 图+音对口型（无 prompt） | 15 | +1080p 0.09/0.05 | |
| `wan2.2animate-v4-motion_retargeting` | 动作迁移（视频参考） | — | 464*832 / 832*464 | 0.04/0.03 |
| `indextts2-v1` | 语音合成（**按次**） | — | — | ￥0.02/次 |

### 4.2 公开 ID 收敛映射

| 公开 ID | 能力 | 量纲 | 承载端点 | 分组（=档位/成本池） |
| --- | --- | --- | --- | --- |
| `minimax-h3-video` | MiniMax H3 视频（文生/图生/首尾帧/多图多音频/对口型） | per_second | 12 | `minimax-h3-video-standard`（默认，lightx2v/b99 系）<br>`minimax-h3-video-fast`（zm_u08 高速版）<br>`minimax-h3-video-pro`（zm_u24 升级画质） |
| `wan-2-2-animate` | 动作迁移 | per_second | 1 | `wan-2-2-animate-standard` |
| `indextts-2` | 语音合成/音色克隆 | **per_call** | 1 | `indextts-2-standard` |

> 13 → 3 个公开 ID。`indextts-2` 必须独立，因为一个公开 ID 不能同时挂按秒与按次渠道（`scripts/ops/lint-catalog-integrity.js:182-192`，P0）。

**保守变体**（若产品要求按形态区分 ID）：文生 `minimax-h3-text-video`、图生 `minimax-h3-image-video`、首尾帧 `minimax-h3-first-last-frame`、多图多音频 `minimax-h3-image-audio-video`。牺牲「单一 ID」原则换取调用方确定性；本设计推荐单 ID + 显式 `mode` 覆盖字段（见 4.4）。

### 4.3 端点选择算法（插件内）

```text
输入：ctx.upstreamModel（分组 pin，可空）、ctx.requestBody、能力矩阵 CAP

1) pin 解析：若 upstreamModel 命中 CAP 的某个 workflow_id → 候选集 = {该端点}（分组强指定，不再按形态切换）
   否则 → 候选集 = 该公开 ID 在该档位下的全部端点（默认全矩阵）
2) 形态判定（按槽位优先级，互斥覆盖）：
      body.ref_video 存在            → motion（动作迁移）
      body.first_frame && last_frame → first-last-frame
      body.ref_audio* 存在 && body.ref_image* 存在 && prompt 缺失 → lip-sync
      body.ref_audio* 存在            → image-audio
      body.ref_image* 存在            → image-reference
      否则                            → text-to-video
3) 能力裁剪：候选 ∩ 形态匹配 ∩ 支持该 resolution ∩ duration ≤ 该端点上限
4) 排序：成本优先（同档位内取进货价最低，其次 7 日成功率/用量），并列时按稳定顺序
5) 无候选 → 400 明确报错（列出该 profile 支持的分辨率/时长上限），绝不静默降级到别的工作流

> **设计取舍**：分组被 pin 后若该档位不支持请求形态（例：`…-fast` 档只有「多图多音频」，却收到纯文生），本设计选择**显式报错**而不是跨档回退。跨档回退会让「选了便宜档却按贵档计费」或反之，破坏调用方对成本的可预期性。若产品要求高可用而非确定性，可改为「回退到同形态的最低成本端点」并在响应头标注实际使用的档位——这需要显式批准，不作为默认。
```

### 4.4 调用方式示例

```http
# 1) 默认（auto）：按令牌的 auto 顺序选档位，插件按请求形态选端点
#    插件型（type 65）视频模型走宿主 openai_video 协议：POST /v1/videos。
#    /v1/video/generations 是原生 TaskAdaptor 通道的路径，对插件模型会 503「无可用渠道」。
#    resolution 用计费档位标签（480p/736p/768p），不是上游端点枚举标签（如 480p竖(480*864)）。
POST /v1/videos
{"model":"minimax-h3-video","prompt":"…","resolution":"768p","duration":5}

# 2) 用分组显式指定档位（等价两种写法）
POST /v1/videos
X-Omnimux-Group: minimax-h3-video-pro
{"model":"minimax-h3-video","prompt":"…","duration":5}

{"model":"minimax-h3-video@minimax-h3-video-fast","prompt":"…"}

# 3) 形态由请求里的媒体槽自动判定，没有独立的 mode 覆盖字段：
#    first_frame + last_frame → 首尾帧；ref_image_N → 多图参考；ref_audio_N（无 prompt）→ 对口型；
#    ref_audio_N + prompt → 多图多音频；仅有 prompt → 文生视频。无候选时 400，不降级。
{"model":"minimax-h3-video","first_frame":"data:image/png;base64,…","last_frame":"data:image/png;base64,…","duration":5}

# 注：801/802/803 三个上游端点自带 mode 参数（高速模式/高质模式，默认高质），
# 它随请求体透传给上游，不参与网关侧形态判定。
```

### 4.5 建议的插件骨架（示意，非最终实现）

```js
export const meta = {
  apiVersion: 1, key: "autodl-comfyui", name: "AutoDL ComfyUI",
  version: "1.0.0", author: { name: "OmniMux" },
  baseUrl: "https://autodl.art", fetchMode: "per_task",
  models: ["minimax-h3-video", "wan-2-2-animate", "indextts-2"],
  usageSchema: { seconds: { type: "number", unit: "second" },
                 resolution: { enum: ["480p","736p","768p","1080p"] } },
  routes: [
    { method: "GET", path: "/api/v1/comfyui/comfyui_workflow/result/{task_id}",
      type: "query", taskIdParam: "task_id", render: "default" }
  ]
};

// 能力矩阵来自 scripts/ops/pricing-ledger/official-autodl-prices.json 的 inputs/prices
const CAP = { /* 见 4.1，含 shapes / resolutions / maxDuration / unitCostCnyPerSec */ };

export function buildSubmitRequest(ctx) {
  const pinned = CAP.byWorkflowId[ctx.upstreamModel];      // 分组 pin
  const shape  = detectShape(ctx.requestBody);             // 请求形态
  const wf     = pickWorkflow(pinned, shape, ctx.requestBody) // 能力裁剪 + 成本优先
    ?? failClosed(shape, ctx.requestBody);
  return {
    url: ctx.baseUrl + "/api/v1/comfyui/comfyui_workflow/" + wf,
    method: "POST",
    body: toUpstreamBody(wf, ctx.requestBody)   // prompt/resolution/duration/ref_* 槽位映射
  };
}

export function buildQueryRequest(ctx) { /* GET result/{ctx.data.task_id} */ }
export function parseSubmitResponse(r) { /* data.task_id → SUBMITTED */ }
export function parseTaskResult(t)     { /* QUEUED/RUNNING/SUCCESS/FAILED，results[] URL */ }
```

---

## 5. 价格模型

### 5.1 可用通道

| 维度 | 机制 | 说明 |
| --- | --- | --- |
| 端点差异（档位） | 分组 `GroupRatio` | 分组 = 成本池；`sell = base × GroupRatio` |
| 分辨率 | usage fact → `OtherRatios` | 以 768p 为基准 1.0：480p≈0.67、736p=1.0、1080p=3.0 |
| 时长 | usage fact `seconds` | 按秒量纲的本体乘子 |
| 上游名自带价 | mapped 上游名的 tiered 表达式 | `relay/relay_task.go:256-263`，可用于「同一公开 ID 下不同端点不同价」 |
| 分时（峰值/空闲） | **无原生原语** | 见 5.3 |

### 5.2 基准价建议（按**恢复原价**，非活动价）

| 档位 | 480P | 768P | 1080P |
| --- | --- | --- | --- |
| 恢复价（9/15 起） | ￥0.03/秒 | ￥0.04/秒 | 未公布 |
| 活动价（至 9/14） | ￥0.02/秒 | ￥0.03/秒 | 0.09–0.10/秒 |

**按恢复价定基准**：活动期内自然获得安全毛利；9/15 后不穿仓。若按活动价定基准，9/15 起 480P 直接亏 33%、768P 亏 25%。

### 5.3 分时价（峰值/空闲）的三个选项

| 选项 | 做法 | 代价 |
| --- | --- | --- |
| **A（推荐）** | **放弃分时**，一律按恢复价峰值定价 | 空闲时段成本红利全部留作毛利；定价可预测、可审计 |
| B | 在插件的 usage hook 里按服务器时间返回 0.5 倍 ratio | 用原生 `OtherRatios`，但价格随挂钟变动；且各档折扣率并不一致（0.5/0.67/0.56/0.6），单乘子无法还原；属 `relay/` 语义面变更需批准 |
| C | 由调用方显式声明 `off_peak: true` 自证 | **禁止**：等于把折扣权交给调用方，必然被套利 |

---

## 6. 落地方案对比

| 方案 | 形态 | 公开 ID 数 | 是否需要 Core-Change | 适用 |
| --- | --- | --- | --- | --- |
| **① 纯配置** | 每个 `(形态, 档位)` 一个渠道，`model_mapping` 指向一个 workflow | 4（按形态拆） | 否 | 当天可上线；13 → 4；但渠道数 ≈ 10–12，运维面大 |
| **② Task Plugin（推荐）** | 1 个插件声明 3 个公开 ID + 路由/能力矩阵，按形态+pin 选 workflow | 3 | **否** | 13 → 3；能力矩阵在插件内可读可测；`relay/` 无改动 |
| ③ 新增 Go 渠道类型 | `relay/channel/task/autodl/` + `constant/` 新类型值 | 3 | **是**（受保护路径，需人工批准与 trailer） | 仅当插件沙箱能力不足（如需要 Go 侧连接/流式框架特性）时 |

**推荐**：② ；若插件沙箱限制（无 `fetch`、同步钩子、单文件）挡住某个必要动作，再退到 ① 的配置兜底，最后才考虑 ③。

---

## 7. 校验与门禁

| 环节 | 动作 |
| --- | --- |
| 命名 | 公开 ID 走 `naming/lint.js`（禁 `_`、`.`、供应商前缀） |
| 目录完整性 | `node scripts/ops/lint-catalog-integrity.js`（含 `BILLING_DIMENSION_CONFLICT`、`MISSING_SELLING_PRICE`） |
| 插件契约 | `new-api plugin lint plugin.js` + `new-api plugin test plugin.js --fixture golden.json`；宿主 `POST /api/plugin/task/:key/dryrun` 干跑钩子 |
| 路由正确性 | fixture 覆盖每种形态 × 每组 × 边界（12s 上限、1080p 仅在支持端点、缺 prompt 的对口型） |
| 计费正确性 | 真实请求对账：标价 × 秒 是否等于上游实际扣费（**待实测**） |
| 生产 | 走蓝绿 + smoke（改的是渠道/插件配置，仍需按 Production 门禁） |

---

## 8. 风险与未决

1. **1080p 双价**：`0.09`（v5、iatv）与 `0.10`（iatv2）并存 → 单 ID 下需按最高价 `0.10` 兜底，或把 iatv2 划入独立档位分组。
2. **736p 视频（b99 系，AI资源仓）恢复价未公布**，当前 0.03/0.02；9/15 后需复核。
3. **活动价 9/14 到期**：定价必须按恢复价建模（见 5.2）。
4. **上游计费口径未实测**：`duration` 是否向上取整、失败是否退款，需一次真实任务对账后才可正式定价。
5. **形态判定的语义边界**：`v2`/`v2_15s` 的 `ref_image_0` 是可选（可纯音频驱动），对口型端点无 `prompt`——fixture 必须覆盖这些非直觉组合。
6. **同档位多端点排序**：默认按进货成本优先，但 `b99_003_12s`（12s 上限）与 `v5`（10s 上限）会因 duration 被裁剪；需在报错文案中明确告知上限，避免调用方误判为模型能力缺失。
7. **插件契约尚未正式发布**：`docs/plugin-api/README.md:3-5` 明确写着「The plugin contract is currently unreleased」。这意味着方案 ② 的钩子签名/元数据形状仍可能变动——接入前应确认目标运行版本是否已冻结 v1；若未冻结，优先用方案 ①（纯配置）上线，插件路线待契约冻结后再切换。这属于**运行时契约风险**，不改变「无需改 `relay/`/`constant/`」的结论。
8. **插件沙箱能力边界**：插件无 `fetch` / imports / async / 环境变量，宿主限制执行时间、并发、输入体积与请求主机；上传插件等同配置渠道凭据（管理员级信任决定），上下线前需审读源码与版本 diff。`meta.allowedHosts` / 渠道 base URL 决定可访问主机，AutoDL 域名需显式放行。

---

## 9. 相关文件

| 文件 | 关系 |
| --- | --- |
| `scripts/ops/pricing-ledger/official-autodl-prices.json` | 能力矩阵/价格真源（本设计 4.1、5.2 的数据来源） |
| `docs/ops/autodl-comfyui-api-survey-zh.md` | 上游调查与接口契约 |
| `docs/plugin-api/README.md` · `v1.d.ts` · `v1.schema.json` | Task Plugin 权威契约 |
| `middleware/distributor.go` · `relay/helper/model_mapped.go` · `relay/relay_task.go` | 分组/映射/计价实现 |
| `docs/ops/model-naming-convention-zh.md` | 公开 ID 命名规范 |
