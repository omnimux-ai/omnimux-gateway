/**
 * Inspiration commands via OmniMux Inspiration API.
 *
 * Routes:
 * - POST /api/inspiration/v1/publish
 * - GET  /api/inspiration/v1/share/:id
 */

const fs = require("fs");
const path = require("path");

const { printJson, UsageError } = require("../lib/output");
const { apiRequest } = require("../lib/http");
const { uploadGatewayFile } = require("../lib/upload");

function parseInspirationFlags(args) {
  const flags = {};
  const positional = [];
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--category") {
      flags.category = args[++i];
    } else if (a.startsWith("--category=")) {
      flags.category = a.slice("--category=".length);
    } else if (a === "--title") {
      flags.title = args[++i];
    } else if (a.startsWith("--title=")) {
      flags.title = a.slice("--title=".length);
    } else if (a === "--prompt") {
      flags.prompt = args[++i];
    } else if (a.startsWith("--prompt=")) {
      flags.prompt = a.slice("--prompt=".length);
    } else if (a === "--model") {
      flags.model = args[++i];
    } else if (a.startsWith("--model=")) {
      flags.model = a.slice("--model=".length);
    } else if (a === "--description" || a === "--desc") {
      flags.description = args[++i];
    } else if (a.startsWith("--description=")) {
      flags.description = a.slice("--description=".length);
    } else if (a.startsWith("--desc=")) {
      flags.description = a.slice("--desc=".length);
    } else if (a === "--media-type") {
      flags.mediaType = args[++i];
    } else if (a.startsWith("--media-type=")) {
      flags.mediaType = a.slice("--media-type=".length);
    } else if (a === "--media-url" || a === "--media") {
      flags.mediaUrl = args[++i];
    } else if (a.startsWith("--media-url=")) {
      flags.mediaUrl = a.slice("--media-url=".length);
    } else if (a.startsWith("--media=")) {
      flags.mediaUrl = a.slice("--media=".length);
    } else if (a === "--cover-url" || a === "--cover") {
      flags.coverUrl = args[++i];
    } else if (a.startsWith("--cover-url=")) {
      flags.coverUrl = a.slice("--cover-url=".length);
    } else if (a.startsWith("--cover=")) {
      flags.coverUrl = a.slice("--cover=".length);
    } else if (a === "--id" || a === "--inspiration-id") {
      flags.id = args[++i];
    } else if (a.startsWith("--inspiration-id=")) {
      flags.id = a.slice("--inspiration-id=".length);
    } else if (a.startsWith("--id=")) {
      flags.id = a.slice("--id=".length);
    } else if (a === "--group") {
      flags.group = args[++i];
    } else if (a.startsWith("--group=")) {
      flags.group = a.slice("--group=".length);
    } else if (a === "--expire") {
      flags.expire = args[++i];
    } else if (a.startsWith("--expire=")) {
      flags.expire = a.slice("--expire=".length);
    } else if (a.startsWith("-")) {
      throw new UsageError(`unknown flag: ${a}`);
    } else {
      positional.push(a);
    }
  }
  return { flags, positional };
}

async function run(ctxOrArgs, authArg, globalFlagsArg) {
  let args = [];
  let auth = authArg;
  let globalFlags = globalFlagsArg || {};

  if (Array.isArray(ctxOrArgs)) {
    args = ctxOrArgs;
  } else if (ctxOrArgs && typeof ctxOrArgs === "object") {
    args = ctxOrArgs.args || [];
    auth = ctxOrArgs.auth || authArg;
    globalFlags = ctxOrArgs.flags || globalFlagsArg || {};
  }

  const { flags, positional } = parseInspirationFlags(args);
  const action = positional[0];

  if (!action || action === "help") {
    printHelp();
    return;
  }

  if (action === "publish") {
    await handlePublish(flags, auth, globalFlags);
    return;
  }

  if (action === "share") {
    const inspirationId = (positional[1] || flags.id || "").trim();
    if (!inspirationId) {
      throw new UsageError(
        "usage: omnimux inspiration share <inspiration_id> [--expire 3days|forever]"
      );
    }
    await handleShare(inspirationId, flags, auth, globalFlags);
    return;
  }

  if (action === "get") {
    const shareId = positional[1];
    if (!shareId) {
      throw new UsageError("usage: omnimux inspiration get <share_id>");
    }
    await handleGet(shareId, auth, globalFlags);
    return;
  }

  if (action === "categories") {
    await handleCategories(flags, auth, globalFlags);
    return;
  }

  throw new UsageError(
    `unknown action: omnimux inspiration ${action}. Available: publish, share, get, categories`
  );
}

async function handlePublish(flags, auth, globalFlags) {
  const category = (flags.category || "").trim();
  if (!category) {
    throw new UsageError(
      "必须包含灵感分类才能发布 (category is required, use --category <category>)"
    );
  }

  const title = (flags.title || "").trim();
  let prompt = (flags.prompt || "").trim();

  // If prompt is a valid file path, load from file
  if (prompt && fs.existsSync(prompt)) {
    try {
      prompt = fs.readFileSync(prompt, "utf-8").trim();
    } catch {
      // keep prompt as string if read fails
    }
  }

  if (!title || !prompt) {
    throw new UsageError(
      "必须包含灵感核心数据(标题与提示词)才能发布 (title and prompt are required, use --title <title> --prompt <prompt>)"
    );
  }

  const rawMedia = (flags.mediaUrl || "").trim();
  const rawCover = (flags.coverUrl || "").trim();

  let mediaUrl = rawMedia;
  let coverUrl = rawCover;

  // If mediaUrl points to a local file, upload it directly to R2 via gateway
  if (mediaUrl && fs.existsSync(mediaUrl)) {
    try {
      const stat = fs.statSync(mediaUrl);
      if (stat.isFile()) {
        const uploaded = await uploadGatewayFile(auth, mediaUrl, "inspiration-videos");
        if (uploaded && (uploaded.file_url || uploaded.download_url)) {
          mediaUrl = uploaded.file_url || uploaded.download_url;
        }
      }
    } catch (err) {
      if (err instanceof UsageError) throw err;
      throw err;
    }
  }

  // If coverUrl points to a local file, upload it directly to R2 via gateway
  if (coverUrl && fs.existsSync(coverUrl)) {
    try {
      const stat = fs.statSync(coverUrl);
      if (stat.isFile()) {
        const uploaded = await uploadGatewayFile(auth, coverUrl, "inspiration-covers");
        if (uploaded && (uploaded.file_url || uploaded.download_url)) {
          coverUrl = uploaded.file_url || uploaded.download_url;
        }
      }
    } catch (err) {
      if (err instanceof UsageError) throw err;
      throw err;
    }
  }

  let mediaType = (flags.mediaType || "").trim();
  if (!mediaType) {
    const candidate = (rawMedia || mediaUrl).toLowerCase();
    if (
      candidate.endsWith(".png") ||
      candidate.endsWith(".jpg") ||
      candidate.endsWith(".jpeg") ||
      candidate.endsWith(".webp") ||
      candidate.endsWith(".gif")
    ) {
      mediaType = "image";
    } else {
      mediaType = "video";
    }
  }

  const payload = {
    source: "local",
    category,
    title,
    prompt,
    model: flags.model || "Universal",
    description: flags.description || "",
    media_type: mediaType,
    media_url: mediaUrl,
    cover_url: coverUrl,
  };
  if (flags.expire) {
    payload.expire = flags.expire;
  }

  const json = await apiRequest(
    auth,
    "POST",
    "/api/inspiration/v1/share",
    payload
  );

  printJson(json.data || json, globalFlags);
}

// A row that already lives in the cloud inspiration library: the same share
// entry, told where the data comes from. Cloud rows are permanent.
async function handleShare(inspirationId, flags, auth, globalFlags) {
  const payload = {
    source: "cloud",
    inspiration_id: inspirationId,
  };
  if (flags.expire) {
    payload.expire = flags.expire;
  }

  const json = await apiRequest(
    auth,
    "POST",
    "/api/inspiration/v1/share",
    payload
  );

  printJson(json.data || json, globalFlags);
}

async function handleGet(shareId, auth, globalFlags) {
  const effectiveAuth =
    auth && auth.baseUrl ? auth : { baseUrl: "https://omnimux.ai" };
  const json = await apiRequest(
    effectiveAuth,
    "GET",
    `/api/inspiration/v1/share/${encodeURIComponent(shareId)}`
  );

  printJson(json.data || json, globalFlags);
}

async function handleCategories(flags, auth, globalFlags) {
  const effectiveAuth =
    auth && auth.baseUrl ? auth : { baseUrl: "https://omnimux.ai" };
  const group = (flags.group || "").trim();
  const query = group ? `?group=${encodeURIComponent(group)}` : "";
  const json = await apiRequest(
    effectiveAuth,
    "GET",
    `/api/inspiration/v1/categories${query}`
  );

  printJson(json.data || json, globalFlags);
}

function printHelp() {
  console.log(`Usage: omnimux inspiration <action> [options]

Actions:
  publish      发布一条灵感内容并生成专属访问链接
  share        为灵感库中已有的灵感生成专属访问链接
  get          获取单条专属灵感详情
  categories   获取灵感社区标准分类字典与参数规则说明

Options for publish:
  --category <string>    灵感分类 (必填: 请使用 categories 接口返回的分类代号或名称)
  --title <string>       灵感标题 (必填: 简明作品标题)
  --prompt <string|file> 提示词内容或文件路径 (必填: 生成该灵感的提示词文本或 JSON 文件)
  --model <string>       关联模型名称 (可选，默认: Universal)
  --desc <string>        描述说明 (可选: 补充背景或效果描述)
  --media <url|file>     媒体直链或本地视频/图片文件 (可选，本地文件自动上传至云端 R2)
  --cover <url|file>     封面图直链或本地图片文件 (可选，本地文件自动上传至云端 R2)
  --media-type <string>  媒体类型: video | image (可选，默认根据文件后缀或 video)
  --expire <string>      有效期: 3days | forever (可选，普通用户本地上传默认 3days，云端灵感默认 forever)

Options for share:
  --expire <string>      有效期: 3days | forever (可选，云端灵感默认永久)

Options for categories:
  --group <string>       按分组过滤: ecommerce | video | lifestyle | creative | general (可选)
`);
}

module.exports = {
  run,
  parseInspirationFlags,
};
