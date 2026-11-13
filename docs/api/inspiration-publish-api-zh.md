# 灵感发布与专属直达接口开发文档 (Inspiration Publishing API)

OmniMux 灵感社区提供面向开发者与创作者的**非公开灵感沉淀与专属链接分享服务**。开发者可通过标准网络接口（API）或本地命令行工具（CLI）直接调用发布接口，实现自动化的灵感资产托管与定向流转。

---

## 1. 核心业务原则与存储分流

- **强制数据门禁**：
  - 发布时**必须同时包含灵感分类（`category`）与灵感核心数据（`title`、`prompt`）**。
  - 缺失任意必填项时，接口在校验阶段立即拒绝并返回友好的中文错误提示，避免残缺数据进入系统。
- **有效期由「数据来源 + 调用者身份」共同决定**：
  - **云端灵感库中已有的灵感**：数据常驻云端灵感库并由灵感库自身对外提供，因此生成的专属链接**永久有效**（`expires_at` 为空），任何登录用户均可生成；
  - **本地上传的数据（调用方自带内容）**：
    - **普通创作者（普通用户）**：受 **3 天（72 小时）保护期限制**（`expires_at = now + 72h`），过期后该专属链接不再可用；
    - **平台运营者（管理员）**：**永久存储且无过期时间**，作为官方标杆案例与长期资产沉淀；
  - 两种来源都可用 `expire` 显式覆盖：`3days` 下调为 72 小时；`forever` 对云端来源或管理员有效，普通用户上传本地数据时请求永久会返回 `403`；
  - 存储标识（`storage_bucket`）随有效期档位：永久档记为 `omnimux-inspiration`，72 小时档记为 `omnimux-files`。
- **「3 天保护」的准确含义**：
  - 对灵感分享记录而言，保护期是**访问控制**——到期后链接返回 `404`，记录本身不会被删除；
  - 真正会删除远端对象的清理任务属于**文件上传能力**：它每 10 分钟扫描 `omnimux-files` 中过期的文件并删除对象。两者不是同一套机制，请勿混用。
- **私密性与全屏直达体验**：
  - 发布的数据**不公开展示**在灵感大厅的主流列表与分类探索中；
  - 发布成功后生成全局唯一的专属直达链接（形如 `https://omnimux.ai/s/insp_xxx`，支持 `/s/:id` 专属短链与 `/inspiration?share=:id` 双重直达）；
  - 任何人通过该链接访问时，前端页面自动 1:1 拉起现有的黑底沉浸式全屏弹窗，完整呈现多媒体播放、结构化提示词排版与创作生成动作。

---

## 2. 对外 API 接口规范

### 2.1 发布灵感内容（兼容别名）

> 推荐直接使用 **§2.3 统一分享入口**。本端点作为兼容别名保留：它等价于统一入口的 `source=local`（本地数据），请求体与响应体完全一致。

向系统提交发布一条灵感资源。支持使用 API Key (`Bearer sk-...`) 或 Web 登录状态调用。

- **请求路径**：`POST /api/inspiration/v1/publish`
- **请求头**：
  - `Authorization: Bearer <API_KEY>`
  - `Content-Type: application/json`
- **请求体参数**：

| 字段 | 类型 | 必填 | 说明 | 示例 |
|---|---|---|---|---|
| `category` | string | **是** | 灵感分类名称（必须包含） | `"电影质感街景"` |
| `title` | string | **是** | 灵感主标题（必须包含） | `"20 世纪 90 年代曼哈顿电影街景"` |
| `prompt` | string | **是** | 提示词核心数据（文本或结构化 JSON 字符串） | `"{\"format\": \"16:9\", ...}"` |
| `model` | string | 否 | 关联的生图/视频模型名称 | `"Seedance 2.5"` |
| `description` | string | 否 | 画面效果与故事描述 | `"30秒电影感曼哈顿街景实拍视频"` |
| `media_type` | string | 否 | 媒体类型，可选 `video` 或 `image`（默认 `video`） | `"video"` |
| `media_url` | string | 否 | 视频或图片媒体网络直链 | `"https://files.omnimux.ai/.../video.mp4"` |
| `cover_url` | string | 否 | 封面图预览链接（若为空且媒体为图片则自动对齐） | `"https://files.omnimux.ai/.../cover.jpg"` |

**请求示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/inspiration/v1/publish" \
  -H "Authorization: Bearer sk-your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "category": "电影质感街景",
    "title": "20 世纪 90 年代曼哈顿电影街景",
    "prompt": "{\"format\":\"16:9, 30s, native audio on\",\"style\":\"1990s Manhattan street cinematography\"}",
    "model": "Seedance 2.5",
    "description": "一个高度详细的结构化 JSON 提示词，用于生成 30 秒曼哈顿电影感视频",
    "media_type": "video",
    "media_url": "https://example.com/manhattan.mp4",
    "cover_url": "https://example.com/cover.jpg"
  }'
```

**响应示例 (200 OK - 普通用户)**：
```json
{
  "success": true,
  "message": "灵感发布成功",
  "data": {
    "share_id": "insp_8e6d425c0e564bde",
    "share_url": "https://omnimux.ai/s/insp_8e6d425c0e564bde",
    "storage_bucket": "omnimux-files",
    "is_admin": false,
    "category": "电影质感街景",
    "title": "20 世纪 90 年代曼哈顿电影街景",
    "expires_at": "2026-09-18T19:30:00+08:00",
    "expires_in": "72h",
    "created_at": "2026-09-15T19:30:00+08:00"
  }
}
```

**响应示例 (200 OK - 平台管理员)**：
```json
{
  "success": true,
  "message": "灵感发布成功",
  "data": {
    "share_id": "insp_15b40cf76a1b2c3d",
    "share_url": "https://omnimux.ai/s/insp_15b40cf76a1b2c3d",
    "storage_bucket": "omnimux-inspiration",
    "is_admin": true,
    "category": "官方标杆案例",
    "title": "官方精选电影镜头",
    "expires_at": null,
    "expires_in": "permanent",
    "created_at": "2026-09-15T19:30:00+08:00"
  }
}
```

---

### 2.2 获取专属灵感详情（公开免登）

通过分享链接携带的 `share_id` 获取该条灵感资源数据。该端点专为前端全屏展示弹窗设计，字段与现有 `PromptItem` 100% 对齐。

- **请求路径**：`GET /api/inspiration/v1/share/:id`
- **鉴权要求**：免登公开访问（内置全局访问限频防护）
- **路径参数**：`:id` 为发布时生成的 `share_id`（例如 `insp_8e6d425c0e564bde`）

**请求示例**：
```bash
curl -sS "https://omnimux.ai/api/inspiration/v1/share/insp_8e6d425c0e564bde"
```

**正常响应 (200 OK)**：
```json
{
  "success": true,
  "data": {
    "id": "insp_8e6d425c0e564bde",
    "slug": "insp_8e6d425c0e564bde",
    "modelKey": "Seedance 2.5",
    "modelLabel": "Seedance 2.5",
    "mediaType": "video",
    "title": "20 世纪 90 年代曼哈顿电影街景",
    "description": "一个高度详细的结构化 JSON 提示词...",
    "prompt": "{\"format\":\"16:9, 30s, native audio on\"...}",
    "category": "电影质感街景",
    "previewImageUrl": "https://example.com/cover.jpg",
    "previewVideoUrl": "https://example.com/manhattan.mp4",
    "posterUrl": "https://example.com/cover.jpg",
    "storage_bucket": "omnimux-files",
    "expires_at": "2026-09-18T19:30:00+08:00",
    "created_at": "2026-09-15T19:30:00+08:00",
    "is_admin": false
  }
}
```

**失效响应 (404 Not Found - 超过 3 天或不存在)**：
```json
{
  "success": false,
  "code": "NOT_FOUND",
  "message": "灵感资源不存在或已超过3天保护期失效 (inspiration share not found or expired)"
}
```

---

## 3. 本地命令行工具 (OmniMux CLI) 使用指南

OmniMux CLI（版本 `>= 0.4.1`）已内置灵感发布模块，方便创作者和开发者在终端一行命令完成发布并取回链接。

### 3.1 检查命令行版本
```bash
omnimux --version
# 输出应包含 cli_version: 0.4.1 或更高
```

### 3.2 一键发布灵感 (`publish`)
```bash
# 1. 基础用法（直接传入提示词文本）
omnimux inspiration publish \
  --category "电影质感街景" \
  --title "20 世纪 90 年代曼哈顿电影街景" \
  --prompt "16:9, 30s, 1990s Manhattan street cinematography, warm Kodak film-stock" \
  --media "https://example.com/preview.mp4"

# 2. 高级用法（直接读取本地 JSON 或提示词文件）
omnimux inspiration publish \
  --category "电商带货实景" \
  --title "夏日清爽饮品商品运镜" \
  --prompt ./my_prompt.json \
  --model "Seedance 2.5" \
  --desc "适合短视频带货的快节奏商品展示分镜" \
  --media "https://example.com/drink_ad.mp4"

# 3. 本地音视频一键直发（自动上传至 Cloudflare R2 并生成短链）
omnimux inspiration publish \
  --category "短剧爆款复刻" \
  --title "古风穿越短剧高潮卡点" \
  --prompt "4k, high quality, historical drama" \
  --media ./output/final_clip.mp4 \
  --cover ./output/cover.jpg
```

**命令行输出**：
```json
{
  "ok": true,
  "data": {
    "share_id": "insp_8e6d425c0e564bde",
    "share_url": "https://omnimux.ai/s/insp_8e6d425c0e564bde",
    "storage_bucket": "omnimux-files",
    "is_admin": false,
    "category": "电商带货实景",
    "title": "夏日清爽饮品商品运镜",
    "expires_at": "2026-09-18T19:30:00+08:00",
    "expires_in": "72h",
    "created_at": "2026-09-15T19:30:00+08:00"
  },
  "meta": {
    "command": "inspiration",
    "cli_version": "0.4.1",
    "contract_version": 1
  }
}
```

### 3.3 查询单条灵感详情 (`get`)
```bash
omnimux inspiration get insp_8e6d425c0e564bde
```

### 3.4 分享灵感库中已有的灵感 (`share`)
```bash
# 为云端灵感库中的灵感生成专属链接（永久有效）
omnimux inspiration share 2789

# 显式指定有效期（管理员可要求永久；普通用户上传本地数据只能 3days）
omnimux inspiration share 2789 --expire 3days
```
`--id` / `--inspiration-id` 与位置参数等价；`publish` 也支持 `--expire`。

---

### 2.3 统一分享入口（云端灵感与本地数据共用）

**`POST /api/inspiration/v1/share`** 是唯一的分享入口：同一份请求体既能分享灵感库中已有的灵感，也能提交调用方自带的本地数据，两者都返回同一个 `/s/:id` 专属链接。

- **鉴权**：用户身份（登录会话或 API Key）
- **请求体**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `source` | string | 否 | `cloud`（云端灵感库已有灵感）或 `local`（调用方自带数据）。省略时按是否提供灵感编号自动推断 |
| `inspiration_id` / `id` | string | 云端必填 | 灵感库中的灵感编号（如 `2789`）；也可以是已生成的分享编号，此时幂等返回原链接 |
| `category` | string | 本地必填 | 灵感分类 |
| `title` | string | 本地必填 | 灵感标题 |
| `prompt` | string | 本地必填 | 提示词或核心数据 |
| `description` | string | 否 | 描述说明 |
| `model` | string | 否 | 关联模型名称，默认 `Universal` |
| `media_type` | string | 否 | `video`（默认）或 `image` |
| `media_url` | string | 否 | 媒体播放直链 |
| `cover_url` | string | 否 | 封面图直链 |
| `expire` | string | 否 | `3days`（72 小时保护期）或 `forever`（永久）。省略时：云端来源永久；本地上传按身份（管理员永久、普通用户 72 小时） |

**云端灵感分享示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/inspiration/v1/share" \
  -H "Authorization: Bearer sk-your-token" \
  -H "Content-Type: application/json" \
  -d '{"source":"cloud","inspiration_id":"2789"}'
```

**本地上传示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/inspiration/v1/share" \
  -H "Authorization: Bearer sk-your-token" \
  -H "Content-Type: application/json" \
  -d '{"source":"local","category":"电影质感街景","title":"90年代曼哈顿","prompt":"cinematic street, 1990s Manhattan"}'
```

**响应示例 (200 OK)**：
```json
{
  "success": true,
  "message": "分享链接已生成",
  "data": {
    "id": "insp_f8482fc14fe64bb1",
    "share_id": "insp_f8482fc14fe64bb1",
    "share_url": "https://omnimux.ai/s/insp_f8482fc14fe64bb1",
    "source": "cloud",
    "permanent": true,
    "expire": "forever",
    "expire_at": null,
    "expires_in": "permanent",
    "storage_bucket": "omnimux-inspiration",
    "is_admin": false,
    "category": "Health & Wellness",
    "title": "情绪共鸣型助眠歌单推广"
  }
}
```

> `is_admin` 表示**发起人的身份**，`permanent`/`expire` 表示**该链接的实际档位**，`source` 表示**数据来源**。以记录为准：幂等重放会返回既有分享的真实档位，而不是本次请求的期望值。

**兼容别名**（保留可用，行为与统一入口一致，建议逐步迁移到 `/share`）：

| 别名 | 等价于 |
|---|---|
| `POST /api/inspiration/v1/publish` | `source=local` 的本地发布 |
| `POST /api/inspiration/v1/inspirations/:id/share` | `source=cloud`，编号取自路径 |

> **失败即失败**：云端编号在上游不存在时返回 `404 NOT_FOUND`，不会返回任何链接。调用方只需判断 `success` 与 `data.share_url`——**拿到链接就一定是可访问的**。

### 2.4 获取灵感社区分类字典与参数规范

向系统查询支持的标准灵感分类列表及发布参数必填/可选规则。公开免登接口，毫秒级响应，适用于本地 Agent、提示词工程联动与前端下拉选择器。

- **请求路径**：`GET /api/inspiration/v1/categories`（或 `GET /api/inspiration/v1/public/categories`）
- **鉴权**：公开访问（无须提供 API Key）
- **查询参数**：
  - `group`（可选）：按类目分组过滤，支持 `ecommerce`（电商营销）、`video`（短视频与短剧）、`lifestyle`（生活垂类）、`creative`（创意设计）、`general`（综合精选）。

**请求示例**：
```bash
# 获取全量分类字典
curl -sS "https://omnimux.ai/api/inspiration/v1/categories"

# 仅获取电商营销类分类
curl -sS "https://omnimux.ai/api/inspiration/v1/categories?group=ecommerce"
```

**响应示例 (200 OK)**：
```json
{
  "success": true,
  "data": {
    "total": 35,
    "groups": [
      { "key": "ecommerce", "name": "电商营销" },
      { "key": "video", "name": "短视频与短剧" },
      { "key": "lifestyle", "name": "生活与垂类" },
      { "key": "creative", "name": "创意艺术" },
      { "key": "general", "name": "综合精选" }
    ],
    "items": [
      {
        "key": "ecommerce-main-image",
        "name": "电商主图",
        "name_en": "E-commerce main image",
        "group": "电商营销",
        "group_key": "ecommerce",
        "description": "适用于电商 Listing、独立站商品主图与白底/场景挂图设计",
        "required_fields": ["category", "title", "prompt"],
        "optional_fields": ["description", "model", "media", "cover", "media_type", "expire"]
      },
      {
        "key": "product-ads",
        "name": "产品广告",
        "name_en": "Product ads",
        "group": "电商营销",
        "group_key": "ecommerce",
        "description": "快节奏带货商品展示短视频、痛点反转与核心卖点高光分镜",
        "required_fields": ["category", "title", "prompt"],
        "optional_fields": ["description", "model", "media", "cover", "media_type", "expire"]
      }
    ],
    "rules": {
      "publish_required_fields": ["category", "title", "prompt"],
      "publish_optional_fields": ["description", "model", "media", "cover", "media_type", "expire"],
      "note": "本地发布必须提供分类 (category)、标题 (title) 与提示词 (prompt)；媒体/封面若为本地文件，CLI 自动上传至 R2"
    }
  }
}
```

---

## 3. 灵感接口参数必填 / 可选全景对照表

为了杜绝客户端传参歧义与遗漏，各场景下字段约束严格如下：

| 字段名 | 类型 | 本地发布 (`publish`) | 云端分享 (`share`) | 查询详情 (`get`) | 说明 |
|---|---|---|---|---|---|
| `category` | string | **必填** | 自动继承 | - | 灵感所属分类（建议从 `categories` 接口动态获取） |
| `title` | string | **必填** | 自动继承 | - | 灵感简明标题 |
| `prompt` | string | **必填** | 自动继承 | - | 提示词核心数据（文本或 JSON 文件内容） |
| `source` | string | 可选 (默认 `local`) | 可选 (默认 `cloud`) | - | 标记数据来源，若提供灵感编号则自动推导为 `cloud` |
| `inspiration_id` / `id` | string | 不适用 | **必填** | **必填** (仅路径) | 云端灵感库中的作品编号（如 `2789`） |
| `media` / `media_url` | string | 可选 | 自动继承 | - | 网络播放直链，或本地音视频文件路径（自动直传 R2） |
| `cover` / `cover_url` | string | 可选 | 自动继承 | - | 封面图直链，或本地图片文件路径（自动直传 R2） |
| `media_type` | string | 可选 (默认 `video` 或按后缀) | 自动继承 | - | `video` 或 `image` |
| `model` | string | 可选 (默认 `Universal`) | 自动继承 | - | 生成该灵感的模型标识 |
| `desc` / `description` | string | 可选 | 自动继承 | - | 额外提示或作品描述 |
| `expire` | string | 可选 (默认按身份) | 可选 (默认 `forever`) | - | `3days`（72 小时保护期）或 `forever`（永久） |

---

## 4. 本地命令行工具 (OmniMux CLI) 使用指南

| 状态码 | 错误代码 | 说明 | 解决建议 |
|---|---|---|---|
| 400 | `INVALID_PARAM` | 缺失必填字段（未提供 `category` 或核心数据 `title`/`prompt`），或该灵感本身缺少分类/标题 | 补齐分类名称与提示词数据后重试 |
| 401 | `UNAUTHORIZED` | 请求未携带有效凭据或凭据已失效 | 检查请求头 `Authorization: Bearer sk-...` 或重新登录 |
| 403 | `FORBIDDEN` | 普通用户为**本地数据**请求了永久有效的分享链接 | 改用 `expire=3days`，或由管理员账号发起（云端灵感库中的灵感默认即永久，不受此限制） |
| 404 | `NOT_FOUND` | 灵感标识不存在，或已超过 3 天保护期被系统清理 | 重新发布新灵感内容，获取新的专属链接 |
| 429 | `TOO_MANY_REQUESTS` | 请求触发了高频防护限流 | 降低请求频率后稍后重试 |
| 503 | `INSPIRATION_UNAVAILABLE` | 灵感服务暂时不可用 | 稍后重试；持续失败请联系运维 |
