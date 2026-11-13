# 文件存储与上传服务开发文档 (File Series API)

OmniMux 文件存储服务基于 **Cloudflare R2（S3 兼容协议）** 与本地安全存储双模驱动构建，接口规范与返回格式深度对齐 [EvoLink.AI 文件服务规范](https://evolink.ai/docs/cn/api-manual/file-series/upload-base64)。

---

## 1. 核心架构与设计原则

- **多后端自动适配 (`pkg/storage`)**：
  - **Cloudflare R2**：基于 AWS SDK Go v2，通过 S3 协议直连 Cloudflare R2 对象存储桶，支持大文件直传、分块及公网 CDN 直链重定向。
  - **本地安全降级**：未配置 R2 环境变量时自动启用本地磁盘存储驱动（`data/storage/`），防止未配置云存储时系统抛错，保障离线与本地开发开箱即用。
- **双端点无缝兼容**：
  - 支持 `/api/v1/files/*`（完全对齐 Evolink `files-api` 路径）
  - 支持 `/v1/files/*`（兼容标准 OpenAI 格式与通用客户端调用）
- **统一身份认证**：
  - 采用 `middleware.TokenOrUserAuth()`，原生支持 **API Key (`Bearer sk-...`)** 与 Web 控制台 **登录 Session** 双模式鉴权。
- **单用户 2000 文件配额门禁**：
  - 每个用户默认最多保留 **2,000** 个活跃（未过期）文件。
  - 每次上传前实时校验活跃文件总数，超出时立即阻断并返回 `HTTP 429`，避免恶意占满存储资源。
- **72 小时生命周期与自动归档清理**：
  - 每个文件写入时精确生成 72 小时过期时间戳：`expires_at = now + 72h`。
  - 主节点运行后台定时巡检协程（`service.StartFileExpirationCleanup`），定期批量扫描过期记录，物理删除 R2/本地对象，并将数据库状态置为已过期。

---

## 2. 环境变量配置 (`.env`)

在应用根目录 `.env` 中配置以下变量即可开启 Cloudflare R2：

```bash
# Cloudflare R2 凭据配置
R2_ACCOUNT_ID=861a8093c2f17a53b4f5a8375a347ff0
R2_ACCESS_KEY_ID=your_r2_access_key_id
R2_SECRET_ACCESS_KEY=your_r2_secret_access_key
R2_BUCKET_NAME=your_r2_bucket_name

# 可选自定义配置
# 自定义 S3 Endpoint（默认根据 R2_ACCOUNT_ID 自动拼接）
R2_ENDPOINT=https://861a8093c2f17a53b4f5a8375a347ff0.r2.cloudflarestorage.com
# R2 绑定的公开访问域名（若配置，file_url 将直接返回此域名直链）
R2_PUBLIC_BASE_URL=https://files.omnimux.ai

# 本地降级驱动的公开访问 Base URL（可选）
FILE_PUBLIC_BASE_URL=https://files.omnimux.ai
```

> **注意**：未配置 `R2_ACCESS_KEY_ID` 或 `R2_BUCKET_NAME` 时，系统自动回退至本地存储模式（文件存储在 `data/storage/`），下载链接将通过网关 `/api/v1/files/download/:file_id` 进行代理回源。

---

## 3. API 接口规范

### 3.1 Base64 文件上传

- **路径**：`POST /api/v1/files/upload/base64`（或 `POST /v1/files/upload/base64`）
- **请求头**：
  - `Authorization: Bearer <API_KEY>`
  - `Content-Type: application/json`
- **请求体**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `base64_data` | string | 是 | 支持 Data URL 格式（如 `data:image/png;base64,iVBORw...`）或纯 Base64 编码 |
| `upload_path` | string | 否 | 自定义分类目录（如 `avatars`、`covers`），默认按 MIME 自动归类 |
| `file_name` | string | 否 | 自定义文件名（如 `avatar.png`），不填时自动生成唯一文件名 |

**请求示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/v1/files/upload/base64" \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "base64_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
    "upload_path": "avatars",
    "file_name": "my_avatar.png"
  }'
```

**响应示例 (200 OK)**：
```json
{
  "success": true,
  "code": 200,
  "msg": "文件上传成功",
  "data": {
    "file_id": "file_8e6d425c0e564bde",
    "file_name": "my_avatar.png",
    "original_name": "my_avatar.png",
    "file_size": 70,
    "mime_type": "image/png",
    "upload_path": "avatars",
    "file_url": "https://omnimux.ai/api/v1/files/download/file_8e6d425c0e564bde",
    "download_url": "https://omnimux.ai/api/v1/files/download/file_8e6d425c0e564bde",
    "upload_time": "2026-09-10T23:49:40+08:00",
    "expires_at": "2026-09-13T23:49:40+08:00"
  }
}
```

---

### 3.2 文件流 / 本地表单上传

- **路径**：`POST /api/v1/files/upload/stream`（或 `POST /v1/files/upload/stream`）
- **请求头**：
  - `Authorization: Bearer <API_KEY>`
  - `Content-Type: multipart/form-data`
- **表单字段**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file` | binary | 是 | 文件二进制数据流（最大支持 100MB） |
| `upload_path` / `uploadPath` | string | 否 | 自定义目录 |
| `file_name` / `fileName` | string | 否 | 自定义文件名（默认取上传文件名） |

**请求示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/v1/files/upload/stream" \
  -H "Authorization: Bearer sk-your-key" \
  -F "file=@photo.png" \
  -F "upload_path=photos" \
  -F "file_name=photo.png"
```

---

### 3.3 URL 远程文件抓取上传

服务端主动抓取远程公开 URL 内容并入库转存，内置私网拦截与 SSRF 安全防护。

- **路径**：`POST /api/v1/files/upload/url`（或 `POST /v1/files/upload/url`）
- **请求头**：
  - `Authorization: Bearer <API_KEY>`
  - `Content-Type: application/json`
- **请求体**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `file_url` | string | 是 | 远程文件的公网可访问 HTTP/HTTPS 链接 |
| `upload_path` | string | 否 | 自定义目录 |
| `file_name` | string | 否 | 自定义文件名 |

**请求示例**：
```bash
curl -sS -X POST "https://omnimux.ai/api/v1/files/upload/url" \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{
    "file_url": "https://example.com/sample.jpg",
    "upload_path": "remote_images",
    "file_name": "sample.jpg"
  }'
```

---

### 3.4 查询当前用户文件配额

- **路径**：`GET /api/v1/files/quota`（或 `GET /v1/files/quota`）
- **请求头**：
  - `Authorization: Bearer <API_KEY>`

**响应示例 (200 OK)**：
```json
{
  "success": true,
  "code": 200,
  "data": {
    "user_group": "default",
    "used_files": 2,
    "max_files": 2000,
    "remain_files": 1998,
    "is_custom": false,
    "quota_reason": ""
  }
}
```

---

### 3.5 文件下载与访问

- **路径**：`GET /api/v1/files/download/:file_id`（或 `GET /v1/files/download/:file_id`）
- **鉴权**：公开免登（通过高熵全局唯一的 `file_id` 寻址，配合 72 小时生命周期保护）
- **特性**：
  - 绑定 R2 公共域名时：自动 302 重定向到 R2 公网 CDN 直链；
  - 本地存储模式时：网关直接流式输出文件内容，自动设置正确的 `Content-Type` 与 `Content-Disposition`；
  - 超过 72 小时或已删除的文件：返回 `404 Not Found`（`{"code":404,"msg":"文件已过期","success":false}`）。

---

## 4. 数据库持久化结构 (`model/file.go`)

GORM 数据表定义（表名 `files`）：

```go
type File struct {
    Id           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
    FileId       string    `json:"file_id" gorm:"type:varchar(64);uniqueIndex;not null"`
    UserId       int       `json:"user_id" gorm:"index;not null"`
    FileName     string    `json:"file_name" gorm:"type:varchar(255);not null"`
    OriginalName string    `json:"original_name" gorm:"type:varchar(255)"`
    FileSize     int64     `json:"file_size" gorm:"not null"`
    MimeType     string    `json:"mime_type" gorm:"type:varchar(128);not null"`
    StorageKey   string    `json:"storage_key" gorm:"type:varchar(512);not null"`
    UploadPath   string    `json:"upload_path" gorm:"type:varchar(255)"`
    FileUrl      string    `json:"file_url" gorm:"type:text;not null"`
    DownloadUrl  string    `json:"download_url" gorm:"type:text"`
    CreatedAt    time.Time `json:"created_at" gorm:"index;not null"`
    ExpiresAt    time.Time `json:"expires_at" gorm:"index;not null"`
    Status       int       `json:"status" gorm:"index;default:1"` // 1: Active, 2: Expired, 3: Deleted
}
```

---

## 5. 常见错误码参考

| HTTP 状态码 | Code | 说明 | 应对方式 |
|---|---|---|---|
| 400 | 400 | 请求体缺失或 Base64 解码失败 | 检查入参格式与 Base64 编码合法性 |
| 401 | 401 | 鉴权失败 / Token 无效 | 检查 `Authorization: Bearer <token>` 是否存在或已失效 |
| 404 | 404 | 文件不存在或已超过 72 小时过期 | 文件已自动清理，请重新上传 |
| 429 | 429 | 用户活跃文件达到 2,000 上限 | 等待旧文件随 72h 周期自动过期释放，或清理已删除文件 |
| 500 | 500 | R2 远端上传异常 | 检查网络连接与 R2 AccessKey / Bucket 配置有效性 |
