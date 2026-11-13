# 生图模型页参考图字段与上限：文档交付报告

- 日期：2026-09-14
- 数据源（只读）：`OmniMux`（`docs/model-governance/specs/*.json`、`openapi` 对应事实、`relay/` 路由事实）
- 编辑对象：`OmniMux-docs`（未提交，工作树保持脏状态供复核）
- 事实授权：限制数字只取自 `docs/model-governance/specs/*.json`；档案留 `null` 的一律写「官方未公布 / not published by the vendor」，不取整、不套用同系列其他模型。

---

## 一、结论（逐页）

### 1.1 生图模型页（`api-reference/image-series/models/`，zh 与 en 同相对路径各一份）

每页做了两件事：请求体表新增参考图字段行；表格后新增「参考图限制」小节。

| 页面（相对路径，`en/`、`zh/` 各一份） | 新增字段行 | 限制小节内容 |
| --- | --- | --- |
| `api-reference/image-series/models/nano-banana-2.mdx` | `images`（string[]，否） | 上限 14、下限无、格式 `image/png,jpeg,webp,heic,heif`、单文件内联 7 MB / GCS 30 MB |
| `api-reference/image-series/models/nano-banana-pro.mdx` | `images`（string[]，否） | 同上（14 / 无下限 / 5 种 MIME / 7 MB·30 MB） |
| `api-reference/image-series/models/qwen-image-3-0.mdx` | `images`（string[]，否） | 上限 3、传图时下限 1、格式 JPG/JPEG/PNG/BMP/TIFF/WEBP/GIF、单文件 10 MB |
| `api-reference/image-series/models/seedream-5-0-pro.mdx` | `images`（string[]，否） | 上限 10、多图生图下限 2（单图生图 1）、格式官方未公布、单文件官方未公布 |
| `api-reference/image-series/models/midjourney.mdx` | `images`（string[]，否） | 张数官方未公布（仅说明可一次用多张）、下限无、格式 `.png/.gif/.webp/.jpg/.jpeg`、单文件官方未公布；并注明版本由提示词 `--v` 选择、`mj-v7` 与 `mj-v8-1` 两份档案格式口径一致 |
| `api-reference/image-series/models/grok-imagine-image.mdx` | `images`（string[]，否） | 无档案 → 只写「能力与上限**未核验**，不声明任何数字」 |
| `api-reference/image-series/models/grok-imagine-image-2-0.mdx` | `images`（string[]，否） | 同上 |
| `api-reference/image-series/models/grok-imagine-image-quality.mdx` | `images`（string[]，否） | 同上 |
| `api-reference/image-series/models/zimage-makeup.mdx` | `images`（string[]，否）行明确写「**本接口不使用**」：App 路径只接受 `metadata.nodeInfoList` 的节点 | 无档案 → 同「未核验，不声明数字」 |

字段行统一写法（en/zh 语义一致）：
`images` = 参考图，图片 URL 或 base64；`image`、`image_urls`、`input_reference` 为可接受的别名，调用上游前统一归一为 `images`。

### 1.2 GPT Image 页（生成器产物）

- 该页由 `scripts/gen-gpt-image-contract.py` 生成，**未手改 MDX**：先改生成器，再重新生成。
- 生成器改动：ops schema 新增 `images`（array of string）；页面正文新增「参考图限制 / Reference image limits」小节。
- 重新生成的文件：
  - `openapi/ops/image/en/gpt-image.json`、`openapi/ops/image/zh/gpt-image.json`（properties 现为 `model, prompt, n, size, quality, images`）
  - `en/api-reference/image-series/gpt-image/generate.mdx`、`zh/api-reference/image-series/gpt-image/generate.mdx`
- 正文结论：`gpt-image-2.5` 单次最多 16 张、无下限、格式与单文件上限官方未公布；`gpt-image-2.5-hd` 无档案 → 不声明；并写明 **50 MB 属于「编辑图 + 其遮罩」口径，不是参考图上限**，故不声明单文件上限。

### 1.3 变更清单

```
 data/changelog/index.json                          | 66 ++++++++++++++++++++-
 data/changelog/pages/1.json                        | 69 +++++++++++++++++++++-
 en|zh/api-reference/image-series/gpt-image/generate.mdx   |  9 +++ (各一份)
 en|zh/api-reference/image-series/models/*.mdx      |  5 ~ 12 ++ (9 模型 × 2 语言)
 openapi/ops/image/en|zh/gpt-image.json             |  7 +++ (各一份)
 scripts/gen-gpt-image-contract.py                  | 18 +++++-
 en/updates.mdx | zh/updates.mdx                    | 31 ++++++++++ (各一份，生成器产物)
 data/changelog/entries/2026-09-14-image-reference-fields.json  (新增)
 27 files changed, 386 insertions(+), 5 deletions(-)
```

### 1.4 变更日志

新增一条 `data/changelog/entries/2026-09-14-image-reference-fields.json`（`type: capability`，`modality: ["image"]`），内容覆盖三点：生图模型页新增参考图字段与上限说明；厂商未公布的项不写数字；**撤回此前的单文件 50 MB 上限（该数字口径是遮罩编辑，不是参考图）**。
`models[]` 只列现网存在的 ID：`gpt-image-2.5`、`mj-v7`、`mj-v8-1`、`nano-banana-2`、`nano-banana-pro`、`qwen-image-3-0`、`seedream-5-0-pro`（依据 `GET https://omnimux.ai/api/pricing` 实测，核验时 103 个模型）。

### 1.5 线路名（wire name）依据

本次没有改动 `openapi/relay.json`（它未建模 `/v1/images/generations` 的参考图字段，只建模了 qwen 形态的 `input.messages[].content[].image` 与 `/v1/images/edits` 的 `ImageEditRequest.image/mask`）。因此字段名以仓内实现为准，并经代码核实：

- `relay/common/relay_info.go`：`TaskSubmitReq{ Image string \`json:"image"\`; Images []string \`json:"images"\`; InputReference string \`json:"input_reference"\` }`
- `relaykit/dto/openai_image.go`：`ImageRequest{ Images json.RawMessage \`json:"images"\`; Image json.RawMessage \`json:"image"\`; Mask ... }`
- `relay/channel/task/sora/grok_imagine_image.go`：`referenceImageURLs()` 接受 `images/images_urls/image/input_reference`，`normalizeImageEditBody()` 统一归一为 `images` 并删除旧键；`isGrokImagineImageName()` 覆盖 `grok-imagine-image*`、`nano-banana-pro`、`nano-banana-2`、`seedream-5-0-pro`、`qwen-image-3-0`、`gpt-image-2.5`
- `relay/channel/task/sora/image_edit_test.go:133`：`POST /v1/images/generations` + `{"model":"gpt-image-2.5","prompt":"a cat","images":["https://a/1.png"]}` 即上述归一化的用例
- `relay/channel/task/gxgenai/parse.go`：`buildRunBody()`（App 路径，`zimage-makeup` 走此路）只组装 `nodeInfoList`，**不消费** `req.Images`；`buildStandardModelBody()`（slug 路径）才把 `images` 映射为 `imageUrls/imageUrl`

---

## 二、证据（每个数字的出处 + 验证命令与实际输出）

### 2.1 数字 → 档案文件

| 页面 | 数字 | 档案文件 | 档案字段 |
| --- | --- | --- | --- |
| nano-banana-2 | 14 / 5 种 MIME / 7 MB（GCS 30 MB） | `specs/nano-banana-2.json` | `reference_images.max=14`、`formats`、`max_bytes_each=7340032` |
| nano-banana-pro | 14 / 5 种 MIME / 7 MB（GCS 30 MB） | `specs/nano-banana-pro.json` | 同上 |
| qwen-image-3-0 | 3 / 1 / 7 种扩展名 / 10 MB | `specs/qwen-image-3-0.json` | `max=3`、`min=1`、`formats`、`max_bytes_each=10485760` |
| seedream-5-0-pro | 10 / 2（单图 1） | `specs/seedream-5-0-pro.json` | `max=10`、`min=2`，`formats=null`、`max_bytes_each=null` |
| midjourney | 5 种扩展名；张数与大小 `null` | `specs/mj-v7.json`、`specs/mj-v8-1.json` | `max=null`、`max_bytes_each=null`、`formats=[png,gif,webp,jpg,jpeg]` |
| gpt-image-2.5 | 16 | `specs/gpt-image-2.5.json` | `reference_images.max=16`（source_claim 为 GPT image 系列口径），`formats=null`、`max_bytes_each=null` |

档案中用于支撑上述数字的原文引述（节选）：

- nano-banana-2 / -pro：`Maximum images per prompt: 14`、`Supported MIME types: image/png , image/jpeg , image/webp , image/heic , image/heif`、`Maximum file size per file for inline data or direct uploads through the console: 7 MB` / `Maximum file size per file from Google Cloud Storage: 30 MB`
- qwen-image-3-0：`不传此参数为文生图（T2I）；传入此参数为图生图（I2I），支持传入 1-3 张图像。`、`图像格式：JPG、JPEG、PNG、BMP、TIFF、WEBP 和 GIF。`、`图像大小：不超过 10MB。`
- seedream-5-0-pro：`多图生图：输入多张参考图片（2-10）+ 文本提示词，生成单张图片。`、`单图生图：输入单张参考图片 + 文本提示词，生成单张图片。`
- mj-v7 / mj-v8-1：`Your image file should end in .png, .gif, .webp, .jpg, or .jpeg.`、`Upload two or more images without text to blend them together...`
- gpt-image-2.5：`Input image references to edit. For GPT image models, you can provide up to 16 images.`

### 2.2 验证命令与实际输出（均在 `OmniMux-docs` 根执行）

```
### 1) python3 scripts/gen-changelog-pages.py
wrote index + 1 page(s) JSON and 2× MDX for 32 entries (Mintlify <Update> layout)
[exit 0]

### 2) python3 scripts/check-changelog.py
2026-03-06-gpt-5-4: model 'gpt-5.4-mini' not on live pricing (document historical evidence or onboard first)
2026-03-08-minimax-m2-5: model 'minimax-m2.5' not on live pricing (document historical evidence or onboard first)
2026-03-08-minimax-m2-5: model 'minimax-m2.7' not on live pricing (document historical evidence or onboard first)
2026-05-20-omni-flash: model 'gemini-omni-flash' not on live pricing (document historical evidence or onboard first)
2026-05-20-veo-3-1: model 'veo-3.1' not on live pricing (document historical evidence or onboard first)
2026-06-01-minimax-m3: model 'minimax-m3' not on live pricing (document historical evidence or onboard first)
2026-08-31-catalog-sync: model 'gpt-4o-mini-tts' not on live pricing (document historical evidence or onboard first)
2026-08-31-media-catalog: model 'gemini-omni-1.1' not on live pricing (document historical evidence or onboard first)
2026-08-31-media-catalog: model 'grok-imagine-image-2-0' not on live pricing (document historical evidence or onboard first)
2026-08-31-media-catalog: model 'pixverse-v6' not on live pricing (document historical evidence or onboard first)
2026-08-31-media-catalog: model 'vidu-q3' not on live pricing (document historical evidence or onboard first)
2026-09-10-gpt-image-25-rename: model 'gpt-image-2.5-hd' not on live pricing (document historical evidence or onboard first)
FAIL: 12 issue(s)
[exit 1]

### 3) python3 scripts/check-i18n.py
check-i18n: OK
[exit 0]

### 4) python3 scripts/check-naming.py
check-naming: OK
[exit 0]

### 5) JSON 校验（技能 W6 项）
docs.json OK
ops + changelog JSON OK
[exit 0]

### 6) 本地内部链接检查（22 个改动 MDX，116 条内部链接）
checked 116 internal link(s) in 22 changed MDX file(s); unresolved: 4
  UNRESOLVED en/updates.mdx: /en/api-reference/image-series/models/gpt-image-2
  UNRESOLVED en/updates.mdx: /en/api-reference/image-series/models/gpt-image-2-hd
  UNRESOLVED zh/updates.mdx: /zh/api-reference/image-series/models/gpt-image-2
  UNRESOLVED zh/updates.mdx: /zh/api-reference/image-series/models/gpt-image-2-hd
```

**第 2 项失败全部为既有问题，与本次变更无关**（逐条核对，12 条中没有一条指向新增条目 `2026-09-14-image-reference-fields`）。剔除本次新增条目后复跑，同样这 12 条仍在（另加 3 条因条目缺失导致的 out-of-sync 报错）：

```
### 把新增条目临时移出后复跑 check-changelog.py（基线）
... 同上 12 条 model membership 报错 ...
index.json items out of sync with entries/ — run gen-changelog-pages.py
index.json total mismatch
1.json out of sync — run gen-changelog-pages.py
FAIL: 15 issue(s)
```

**第 6 项的 4 条断链也是既有问题**：`git show HEAD:en/updates.mdx | grep -c "models/gpt-image-2"` → `2`（zh 同为 `2`），来自历史条目 `2026-04-28-gpt-image-2` 的链接，由生成器原样重排，非本次引入。

---

## 三、未知（未能写出的限制及原因）

| 模型 | 未能声明的项 | 原因 |
| --- | --- | --- |
| `grok-imagine-image` / `-2-0` / `-quality` | 张数、格式、单文件大小（全部） | `docs/model-governance/specs/` 下**没有任何** grok-imagine-image 档案（`grep -rl "grok-imagine-image\|zimage" docs/model-governance/` 无命中）。按诚实规则，无档案不声明限制；页面明写能力「未核验」 |
| `zimage-makeup` | 参考图张数、格式、单文件大小（全部） | 同上无档案；且其 App 路径本身不经过顶层参考图字段，媒体只能以节点传入，节点 ID 取决于上游应用，未核验 |
| `gpt-image-2.5-hd` | 张数、格式、单文件大小（全部） | 无该 ID 的档案（档案是 `gpt-image-2.5` / `-flare` / `-sunburst`）；不从同系列档案套用数字 |
| `seedream-5-0-pro` | 接受格式、单文件大小 | 档案 `formats=null`、`max_bytes_each=null`（官方参数表只写 `image string / string[]`，未枚举格式；无字节口径） |
| `midjourney` | 张数上限、单文件大小 | `mj-v7` / `mj-v8-1` 两档案均为 `null`（官方只描述「两张以上」用法，未公布上限；无字节口径） |
| `gpt-image-2.5` | 接受格式、单文件大小 | 档案两项皆 `null`：官方只列 base64 / URL / file_id 三种传入方式，未枚举 MIME；50 MB 一句口径属于「编辑图 + 遮罩」，不是单文件参考图上限 |
| 全部页面 | 各渠道实际可接受量 | 档案记录的是**厂商上限**；中转渠道可能更严，故页面用「厂商口径」表述，不承诺线路行为 |

补充事实（供治理判断，本次未改动产品仓）：`web/src/features/model-playground/lib/param-schema.ts` 的 `REFERENCE_IMAGE_LIMITS` 给 `grok-imagine-image-2-0` 写了 `maxCount: 5`，但该模型**没有任何模型档案**支撑该数字，注释同样标注为「xAI Imagine: up to 5 reference images」。这与 PLAN.md 记载的第 1 类缺失（数字无出处）同源；另外该表把 `seedream-5-0-pro` 记为 10（档案 max=10，但档案 min=2），`mj-*`、`zimage-makeup` 则未覆盖。

---

## 四、未覆盖（有意未动）

1. **`docs.json` 导航**：未改。本次没有新增页面，现有 9 个模型叶与 1 个 GPT Image 叶均在导航中，无需新叶。
2. **`openapi/relay.json`**：未改。它不是本次权威源，且按技能约定由 `scripts/sync-openapi.py` 同步；参考图字段缺建模一事已在正文与「未知」中登记。
3. **`openapi/ops/**` 中非 GPT Image 的 ops 文件**：未改（模型页用的是 `api:` frontmatter 内联契约，不是 ops 文件）。
4. **产品仓 `OmniMux`**：除本报告外**未写入任何文件**。核验：`git status --short` 在我开始前已有他人未提交改动，且文件修改时间早于本任务开始时间（本任务约 14:15 开始）：
   - ` M docs/model-governance/CONTRACT.md`（mtime 14:13:14）
   - `?? docs/model-governance/CURRENT-STATE.html`（mtime 14:11:16）
   - `?? docs/product/adaptive-playground-form-research-zh.md`（mtime 2026-09-13 10:49:42）
   - `?? docs/product/inspiration-clean-nav-preview.html`（mtime 2026-09-13 13:36:21）
   - 本次唯一新增：`?? docs/model-governance/reports/`（即本报告）
   `web/src/features/model-playground/lib/param-schema.ts` 的 `maxCount: 5` 仅作发现记录，未改代码。
5. **历史 changelog 条目的 12 条 membership 失败与 4 条断链**：未修（超出本次范围，且属历史事实与既有产物，需单独授权与取证）。
6. **`specs/` 档案本身**：未改。grok / zimage 档案缺失是本次「未知」的根因之一，但建档属于另一条治理线（档案 → 门禁 → 目录 → 文档），未在本任务内创建。
7. **未提交、未推送**：docs 仓保持脏工作树；无 worktree、无分支切换。
