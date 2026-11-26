const SORA_MODELS = ["sora-2", "sora-2-pro"];
const MUSE_VIDEO_MODEL = "muse-video";
const SEEDANCE_VIDEO_DECODER = "seedance-2-5-openai-decoder";
const SEEDANCE_FLASH_CA_RESOLUTIONS = {
  "sd2.5-ca-flash-480p": "480p",
  "sd2.5-ca-flash-720p": "720p",
};
const SEEDANCE_FLASH_FIXED_720P_MODELS = ["jiuyue-sd25-flash-720p"];
const SEEDANCE_FLASH_FIXED_RATIOS = ["16:9", "9:16", "1:1", "4:3", "3:4", "21:9"];

export const meta = {
  apiVersion: 1,
  key: "sora",
  name: "Sora",
  icon: "Sora.Color",
  description: {
    en: "OpenAI Sora video generation (text-to-video, image-to-video, and remix)",
    zh: "OpenAI Sora 视频生成（文生视频、图生视频、remix）",
  },
  version: "1.1.0",
  channelTypes: [55, 1], // OpenAI-type channels natively serve sora with the same wire format
  author: { name: "QuantumNous" },
  models: [...SORA_MODELS, MUSE_VIDEO_MODEL],
  fetchMode: "per_task",
  // A New API gateway serves /v1/videos as a host protocol, so no URL changes.
  upstreams: ["vendor", "new_api"],
  usageSchema: {
    // Requested video duration in seconds.
    seconds: {
      type: "number",
      unit: "second",
      description: { en: "Video generation unit price", zh: "视频生成单价" },
    },
    // Requested output video dimensions.
    size: {
      enum: ["720x1280", "1280x720", "1792x1024", "1024x1792"],
      enumLabels: {
        "720x1280": { en: "720x1280", zh: "720x1280" },
        "1280x720": { en: "1280x720", zh: "1280x720" },
        "1792x1024": { en: "1792x1024", zh: "1792x1024" },
        "1024x1792": { en: "1024x1792", zh: "1024x1792" },
      },
      description: { en: "Output video dimensions", zh: "输出视频尺寸" },
    },
  },
  protocols: [{ name: "openai_responses", models: SORA_MODELS, supports: ["stream", "sync", "background"] }, "openai_video"],
};

function trimmed(value) {
  return String(value || "").trim();
}

function responsesInput(req) {
  const texts = [],
    images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!item || typeof item !== "object" || Array.isArray(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!part || typeof part !== "object" || Array.isArray(part)) continue;
        if (["input_text", "text"].includes(part.type) && typeof part.text === "string") texts.push(part.text);
        if (["input_image", "image_url"].includes(part.type)) {
          let image = part.image_url;
          if (image && typeof image === "object") image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return {
    prompt: texts
      .filter(function (text) {
        return trimmed(text);
      })
      .join("\n"),
    images: images,
  };
}

function responsesVideoText(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

function requestValues(req, model) {
  const values = Object.assign({}, req || {});
  values.model = model;
  return values;
}

function seedanceFlashCAResolution(upstreamModel) {
  return SEEDANCE_FLASH_CA_RESOLUTIONS[trimmed(upstreamModel).toLowerCase()] || "";
}

function isSeedanceFlashFixed720pUpstream(upstreamModel) {
  return SEEDANCE_FLASH_FIXED_720P_MODELS.includes(trimmed(upstreamModel).toLowerCase());
}

function isSeedanceFlashOpenAIUpstream(upstreamModel) {
  return Boolean(seedanceFlashCAResolution(upstreamModel)) || isSeedanceFlashFixed720pUpstream(upstreamModel);
}

function parseStrictSeedanceFlashInt(value) {
  if (typeof value === "number") return Number.isInteger(value) ? value : NaN;
  if (typeof value === "string") {
    const text = value.trim();
    if (/^-?\d+$/.test(text)) return Number(text);
  }
  return NaN;
}

function parseSeedanceFlashSeconds(req) {
  const hasSeconds = req.seconds !== undefined && req.seconds !== null && String(req.seconds).trim() !== "";
  const hasDuration = req.duration !== undefined && req.duration !== null && String(req.duration).trim() !== "";
  if (!hasSeconds && !hasDuration) return 5;
  const parsed = [];
  if (hasSeconds) parsed.push(parseStrictSeedanceFlashInt(req.seconds));
  if (hasDuration) parsed.push(parseStrictSeedanceFlashInt(req.duration));
  if (parsed.some(v => !Number.isInteger(v) || v < 4 || v > 30)) {
    throw new Error("Seedance 2.5 Flash duration must be an integer from 4 to 30 seconds");
  }
  if (parsed.length > 1 && parsed[0] !== parsed[1]) {
    throw new Error("duration and seconds must specify the same integer from 4 to 30");
  }
  return parsed[0];
}

function parseUrlList(value, fieldName) {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) throw new Error(fieldName + " must be an array of URLs");
  return value.map(item => {
    if (typeof item !== "string" || !item.trim()) {
      throw new Error(fieldName + " entries must be non-empty strings");
    }
    return item.trim();
  });
}

function parsePixelSize(raw) {
  const normalized = trimmed(raw).toLowerCase().replace(/[×*]/g, "x").replace(/\s+/g, "");
  const match = /^(\d+)x(\d+)$/.exec(normalized);
  if (!match) return null;
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0) return null;
  const shorter = Math.min(width, height);
  const resolution = shorter === 480 ? "480p" : shorter === 720 ? "720p" : "";
  if (!resolution) return null;
  const ratio = width > height ? "16:9" : height > width ? "9:16" : "1:1";
  return { resolution, ratio };
}

function assertSeedanceFlashResolutionMatches(req, skuResolution, pixelRes) {
  const resText = trimmed(req.resolution).toLowerCase();
  const qualText = trimmed(req.quality).toLowerCase();
  if (resText && qualText && resText !== qualText) {
    throw new Error("resolution and quality must match when both are provided");
  }
  const declared = resText || qualText;
  if (declared && declared !== skuResolution) {
    throw new Error("resolution " + declared + " does not match " + skuResolution + " route");
  }
  if (pixelRes && pixelRes !== skuResolution) {
    throw new Error("size resolution " + pixelRes + " does not match " + skuResolution + " route");
  }
}

function rejectUnsupportedSeedanceFlashOperations(req) {
  const op = trimmed(req.operation).toLowerCase();
  const taskType = trimmed(req.omni_reference_task_type).toLowerCase();
  if (op === "video_edit" || op === "video_extend" || taskType === "edit" || taskType === "extend") {
    throw new Error("video_edit and video_extend are not supported on this route");
  }
}

function buildSeedanceFlashMultipartParts(values, files) {
  const parts = [];
  for (const key of Object.keys(values)) {
    const val = values[key];
    if (val === undefined || val === null) continue;
    if (Array.isArray(val)) {
      parts.push({ name: key, value: JSON.stringify(val) });
    } else if (typeof val === "object") {
      parts.push({ name: key, value: JSON.stringify(val) });
    } else {
      parts.push({ name: key, value: val });
    }
  }
  for (const file of files) parts.push({ name: file.field, fileRef: file.ref, filename: file.filename });
  return parts;
}

function buildSeedanceFlashCABody(req, upstreamModel, skuResolution, files) {
  const prompt = String(req.prompt || "");
  if (!prompt.trim()) throw new Error("field prompt is required");
  if (Array.from(prompt).length > 10000) {
    throw new Error("prompt exceeds 10000 characters for this route");
  }
  rejectUnsupportedSeedanceFlashOperations(req);
  const seconds = parseSeedanceFlashSeconds(req);

  const rawAspect = trimmed(req.aspect_ratio);
  const rawSize = trimmed(req.size);
  const pixel = parsePixelSize(rawSize);
  assertSeedanceFlashResolutionMatches(req, skuResolution, pixel ? pixel.resolution : "");

  const effectiveSizeRatio = pixel ? pixel.ratio : rawSize;
  if (rawAspect && effectiveSizeRatio && rawAspect !== effectiveSizeRatio && rawAspect !== "adaptive") {
    throw new Error("aspect_ratio and size must match when both are provided");
  }
  const ratio = rawAspect || effectiveSizeRatio;

  const imageSources = ["reference_image_urls", "image_urls", "images"].filter(k => req[k] !== undefined && req[k] !== null);
  if (imageSources.length > 1) throw new Error("supply only one reference image array");
  const refImages = imageSources.length ? parseUrlList(req[imageSources[0]], imageSources[0]) : [];

  const videoSources = ["reference_video_urls", "video_urls"].filter(k => req[k] !== undefined && req[k] !== null);
  if (videoSources.length > 1) throw new Error("supply only one reference video array");
  const refVideos = videoSources.length ? parseUrlList(req[videoSources[0]], videoSources[0]) : [];

  const audioSources = ["reference_audio_urls", "audio_urls"].filter(k => req[k] !== undefined && req[k] !== null);
  if (audioSources.length > 1) throw new Error("supply only one reference audio array");
  const refAudios = audioSources.length ? parseUrlList(req[audioSources[0]], audioSources[0]) : [];

  let firstFrame = trimmed(req.input_reference);
  let lastFrame = trimmed(req.last_frame);
  if (req.image_with_roles !== undefined && req.image_with_roles !== null) {
    if (!Array.isArray(req.image_with_roles)) throw new Error("image_with_roles must be an array");
    for (const row of req.image_with_roles) {
      if (!row || typeof row !== "object") throw new Error("image_with_roles entries must be objects");
      const url = typeof row.url === "string" ? row.url.trim() : "";
      if (!url) throw new Error("image_with_roles url is required");
      const role = trimmed(row.role).toLowerCase();
      if (role === "first" || role === "first_frame") {
        if (firstFrame) throw new Error("at most one first_frame image is allowed");
        firstFrame = url;
      } else if (role === "last" || role === "last_frame") {
        if (lastFrame) throw new Error("at most one last_frame image is allowed");
        lastFrame = url;
      } else if (role === "reference" || role === "reference_image") {
        refImages.push(url);
      } else {
        throw new Error("unsupported image_with_roles role: " + role);
      }
    }
  }
  const hasUploadedFiles = (files || []).length > 0;
  if (lastFrame && !firstFrame && !hasUploadedFiles) throw new Error("last_frame requires first_frame");
  const hasFrames = Boolean(firstFrame || lastFrame || hasUploadedFiles);
  const totalRefs = refImages.length + refVideos.length + refAudios.length;
  if (hasFrames && totalRefs > 0) {
    throw new Error("frame inputs and reference media arrays are mutually exclusive");
  }
  if (refImages.length > 10 || refVideos.length > 5 || refAudios.length > 5 || totalRefs > 20) {
    throw new Error("reference media exceeds 10 images / 5 videos / 5 audios (total 20)");
  }

  let normalizedRatio = ratio;
  if (normalizedRatio === "adaptive") {
    if (hasFrames) {
      // Frame-locked gateway contracts send aspect_ratio="adaptive"; omit so upstream follows the frame.
      normalizedRatio = "";
    } else {
      throw new Error("aspect_ratio adaptive is not supported on this route; use one of " + SEEDANCE_FLASH_FIXED_RATIOS.join(", "));
    }
  } else if (normalizedRatio && !SEEDANCE_FLASH_FIXED_RATIOS.includes(normalizedRatio)) {
    throw new Error("aspect_ratio " + normalizedRatio + " is not supported on this route; use one of " + SEEDANCE_FLASH_FIXED_RATIOS.join(", "));
  }

  const body = requestValues(req, upstreamModel);
  for (const key of [
    "operation",
    "omni_reference_task_type",
    "duration",
    "size",
    "quality",
    "image_with_roles",
    "image_urls",
    "images",
    "video_urls",
    "audio_urls",
    "reference_image_urls",
    "reference_video_urls",
    "reference_audio_urls",
    "input_reference",
    "last_frame",
    "output_format",
    "watermark",
    "return_last_frame",
    "nsfw_check",
    "draft",
    "draft_task_id",
    "group",
  ]) {
    delete body[key];
  }
  body.seconds = seconds;
  body.resolution = skuResolution;
  if (normalizedRatio) body.aspect_ratio = normalizedRatio;
  else delete body.aspect_ratio;
  if (firstFrame) body.input_reference = firstFrame;
  if (lastFrame) body.last_frame = lastFrame;
  if (refImages.length) body.reference_image_urls = refImages;
  if (refVideos.length) body.reference_video_urls = refVideos;
  if (refAudios.length) body.reference_audio_urls = refAudios;
  return body;
}

function buildSeedanceFlashFixed720pBody(req, upstreamModel, files) {
  if (!String(req.prompt || "").trim()) throw new Error("field prompt is required");
  rejectUnsupportedSeedanceFlashOperations(req);
  const seconds = parseSeedanceFlashSeconds(req);
  const rawAspect = trimmed(req.aspect_ratio);
  const rawSize = trimmed(req.size);
  const pixel = parsePixelSize(rawSize);
  assertSeedanceFlashResolutionMatches(req, "720p", pixel ? pixel.resolution : "");

  const effectiveSizeRatio = pixel ? pixel.ratio : rawSize;
  if (rawAspect && effectiveSizeRatio && rawAspect !== effectiveSizeRatio) {
    throw new Error("aspect_ratio and size must match when both are provided");
  }
  const ratio = rawAspect || effectiveSizeRatio;
  const allowed720pRatios = SEEDANCE_FLASH_FIXED_RATIOS.concat(["adaptive"]);
  if (ratio && !allowed720pRatios.includes(ratio)) {
    throw new Error("aspect_ratio " + ratio + " is not supported on this route");
  }

  const body = requestValues(req, upstreamModel);
  for (const key of [
    "operation",
    "omni_reference_task_type",
    "duration",
    "quality",
    "size",
    "output_format",
    "watermark",
    "return_last_frame",
    "nsfw_check",
    "draft",
    "draft_task_id",
    "group",
  ]) {
    delete body[key];
  }
  body.seconds = seconds;
  body.resolution = "720p";
  if (ratio) body.aspect_ratio = ratio;
  else delete body.aspect_ratio;
  return body;
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  if (!String(req.prompt || "").trim()) throw new Error("field prompt is required");
  const action = ctx.action === "remix" ? "remix" : ctx.action;
  const headers = { Authorization: "Bearer " + ctx.apiKey };
  if (action === "remix") {
    headers["Content-Type"] = "application/json";
    return { url: ctx.baseUrl + "/v1/videos/" + ctx.originTaskId + "/remix", method: "POST", headers, body: requestValues(req, ctx.upstreamModel), action };
  }
  const files = ctx.files || [];
  const caRes = seedanceFlashCAResolution(ctx.upstreamModel);
  if (caRes) {
    const body = buildSeedanceFlashCABody(req, ctx.upstreamModel, caRes, files);
    if (files.length) {
      return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, bodyType: "multipart", parts: buildSeedanceFlashMultipartParts(body, files) };
    }
    headers["Content-Type"] = "application/json";
    return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, body };
  }
  if (isSeedanceFlashFixed720pUpstream(ctx.upstreamModel)) {
    const body = buildSeedanceFlashFixed720pBody(req, ctx.upstreamModel, files);
    if (files.length) {
      return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, bodyType: "multipart", parts: buildSeedanceFlashMultipartParts(body, files) };
    }
    headers["Content-Type"] = "application/json";
    return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, body };
  }
  if ((ctx.files || []).length) {
    const parts = [];
    const values = requestValues(req, ctx.upstreamModel);
    for (const key of Object.keys(values)) {
      if (values[key] !== undefined && values[key] !== null && typeof values[key] !== "object") parts.push({ name: key, value: values[key] });
    }
    if (values.metadata && typeof values.metadata === "object" && !Array.isArray(values.metadata)) {
      parts.push({ name: "metadata", value: JSON.stringify(values.metadata) });
    }
    for (const file of ctx.files) parts.push({ name: file.field, fileRef: file.ref, filename: file.filename });
    return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, bodyType: "multipart", parts };
  }
  headers["Content-Type"] = "application/json";
  return { url: ctx.baseUrl + "/v1/videos", method: "POST", headers, body: requestValues(req, ctx.upstreamModel) };
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const taskId = body.id || body.task_id;
  if (!taskId) throw new Error("task_id is empty");
  return { taskId, taskData: body };
}

function isSeedanceEconomyVideo(ctx) {
  return ctx.action === "video_multi_ref" && String(ctx.model || "").toLowerCase().replace(/\./g, "-") === "seedance-2-5";
}

export function extractUsage(ctx) {
  if (ctx.action === "remix") return {};
  const req = ctx.requestBody || {};
  if (isSeedanceEconomyVideo(ctx)) {
    // The decoder validates seconds. The resolution tier does not establish output dimensions.
    return { seconds: req.seconds };
  }
  if (isSeedanceFlashOpenAIUpstream(ctx.upstreamModel)) {
    return { seconds: parseSeedanceFlashSeconds(req) };
  }
  let seconds = Number(req.seconds || req.duration || 4);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = 4;
  return { seconds: Math.min(seconds, 3600), size: req.size || "720x1280" };
}

export function extractUsageOnComplete(task, taskResult, body) {
  const facts = {};
  const payload = body || {};
  const upstreamModel =
    trimmed(payload.model) ||
    trimmed(task && (task.upstreamModel || task.upstream_model || (task.properties && task.properties.upstream_model_name)));
  if (isSeedanceFlashOpenAIUpstream(upstreamModel)) {
    // Ignore ambiguous outer `duration` (often wall-clock elapsed time); accept explicit media seconds only.
    const seconds = Number(payload.seconds || payload.video_duration || payload.actual_duration || 0);
    if (Number.isFinite(seconds) && seconds > 0) facts.seconds = Math.min(seconds, 3600);
    return facts;
  }
  const seconds = Number(payload.seconds || payload.duration || 0);
  if (Number.isFinite(seconds) && seconds > 0) facts.seconds = Math.min(seconds, 3600);
  const size = trimmed(payload.size);
  if (["720x1280", "1280x720", "1792x1024", "1024x1792"].includes(size)) facts.size = size;
  return facts;
}

function isAsyncImageModel(model) {
  const m = String(model || "").toLowerCase();
  return (
    m.includes("image") ||
    m.includes("seedream") ||
    m.includes("banana") ||
    m.includes("dall-e") ||
    ["nano-banana-pro", "nano-banana-2", "nano-banana-2-pro", "seedream-5-0-pro", "qwen-image-3-0", "gpt-image-2.5", "gemini-3.1-flash-image", "nano-banana-2-lite"].includes(m)
  );
}

// firstUrl digs a media URL out of string / array / object shapes. Upstreams
// disagree on this: APIMart's completed image payload carries the URL as
// result.images[0].url = ["https://…"] (an ARRAY of one), Evolink-style flat
// payloads use result.images = ["https://…"], and some use a bare string.
function firstUrl(value) {
  if (typeof value === "string") return value.trim();
  if (Array.isArray(value)) {
    for (const item of value) {
      const found = firstUrl(item);
      if (found) return found;
    }
    return "";
  }
  if (value && typeof value === "object") {
    for (const key of ["url", "image_url", "video_url"]) {
      const found = firstUrl(value[key]);
      if (found) return found;
    }
  }
  return "";
}

function artifactImageUrl(data) {
  if (!data) return "";
  const candidates = [
    data.results,
    data.result_data,
    data.result && data.result.images,
    data.images,
    data.data && data.data.images,
    data.data,
  ];
  for (const candidate of candidates) {
    const url = firstUrl(candidate);
    if (url) return url;
  }
  return "";
}

// unwrapDataEnvelope returns the payload that actually carries status / progress /
// result. APIMart nests them one level down ({"code":200,"data":{…}}) while
// Evolink-style upstreams keep them flat. `data` is unwrapped only when it is an
// OBJECT: the submit response uses an ARRAY there, and unwrapping that would throw
// the envelope away.
function unwrapDataEnvelope(body) {
  if (!body || typeof body !== "object") return {};
  const data = body.data;
  if (data && typeof data === "object" && !Array.isArray(data)) return data;
  return body;
}

function artifactVideoUrl(data) {
  if (!data) return "";
  for (const value of [data.video_url, data.result_url, data.url]) {
    const url = firstUrl(value);
    if (/^https?:\/\/[^\s/?#]+(?:[/?#]|$)/i.test(url)) return url;
  }
  return "";
}

function isImageTaskData(data) {
  if (!data) return false;
  if (data.type === "image" || data.object === "image.generation.task") return true;
  if (firstUrl(data.video_url)) return false;
  if (artifactImageUrl(data) !== "") return true;
  return false;
}

export function buildQueryRequest(ctx) {
  if (!isSeedanceEconomyVideo(ctx) && (isAsyncImageModel(ctx.model) || isAsyncImageModel(ctx.upstreamModel))) {
    return { url: ctx.baseUrl + "/v1/tasks/" + ctx.taskId, method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
  }
  return { url: ctx.baseUrl + "/v1/videos/" + ctx.taskId, method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
}

export function parseTaskResult(ctx, body) {
  const statuses = {
    queued: "QUEUED",
    pending: "QUEUED",
    submitted: "QUEUED",
    processing: "IN_PROGRESS",
    in_progress: "IN_PROGRESS",
    completed: "SUCCESS",
    succeeded: "SUCCESS",
    failed: "FAILURE",
    error: "FAILURE",
    cancelled: "FAILURE",
  };
  const envelope = unwrapDataEnvelope(body);
  const rawStatus = envelope.status !== undefined ? envelope.status : body && body.status;
  const mapped = statuses[String(rawStatus || "").toLowerCase()];
  const result = { status: mapped || "UNKNOWN" };
  if (!mapped) result.reason = "unrecognized status: " + String(rawStatus || "");
  const progress = Number(envelope.progress !== undefined ? envelope.progress : body && body.progress);
  if (Number.isFinite(progress) && progress > 0 && progress < 100) result.progress = progress + "%";
  if (result.status === "FAILURE") {
    const err = (envelope && envelope.error) || (body && body.error);
    result.reason = err && err.message ? err.message : "task failed";
  }
  if (result.status === "SUCCESS") {
    const rawUrl = artifactVideoUrl(envelope) || artifactVideoUrl(body) || artifactImageUrl(envelope) || artifactImageUrl(body);
    if (rawUrl) {
      result.url = rawUrl;
    }
  }
  return result;
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  const data = unwrapDataEnvelope(task.data);
  if (isImageTaskData(data)) {
    return [{ key: "image", type: "image", mimeType: "image/jpeg" }];
  }
  return [{ key: "video", type: "video" }];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey === "image") {
    const data = unwrapDataEnvelope(ctx.data);
    const imgUrl = artifactImageUrl(data);
    if (!imgUrl) throw new Error("artifact_not_found");
    return { url: imgUrl, method: ctx.clientRequest.method, credentialless: true };
  }
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  const videoUrl = artifactVideoUrl(unwrapDataEnvelope(ctx.data)) || artifactVideoUrl(ctx.data);
  if (videoUrl && !(ctx.upstream && ctx.upstream.kind === "new_api"))
    return { url: videoUrl, method: ctx.clientRequest.method, credentialless: true };
  return {
    url: ctx.baseUrl + "/v1/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/content",
    method: ctx.clientRequest.method,
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const req = ctx.body.value;
      if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      const model = trimmed(req.model);
      if (!model) throw new Error("model is required");
      if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
      if (req.images !== undefined && !Array.isArray(req.images)) throw new Error("images must be an array");
      if (req.metadata !== undefined && (!req.metadata || typeof req.metadata !== "object" || Array.isArray(req.metadata)))
        throw new Error("metadata must be an object");
      const input = responsesInput(req);
      const prompt = input.prompt || trimmed(req.prompt);
      if (!prompt) throw new Error("input is required");
      const images = [];
      for (const image of [req.image, req.input_reference].concat(req.images || [], input.images)) {
        if (trimmed(image) && !images.includes(trimmed(image))) images.push(trimmed(image));
      }
      const requestBody = { model: model, prompt: prompt };
      if (images.length) requestBody.input_reference = images[0];
      if (Object.prototype.hasOwnProperty.call(req, "seconds")) requestBody.seconds = req.seconds;
      else if (Object.prototype.hasOwnProperty.call(req, "duration")) requestBody.seconds = req.duration;
      if (Object.prototype.hasOwnProperty.call(req, "size")) requestBody.size = req.size;
      if (Object.prototype.hasOwnProperty.call(req, "metadata")) requestBody.metadata = req.metadata;
      return { kind: "submit", model: model, action: images.length ? "image_to_video" : "text_to_video", requestBody: requestBody };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const value = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesVideoText(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE")
        return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }], state: state, done: true };
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx, _task) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesVideoText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "sora" },
      };
    },
  },
};

const legacyRenderers = {
  openai_video: function (task) {
    const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
    const output = {
      id: task.task_id,
      object: "video",
      model: (task.properties || {}).origin_model_name || "",
      status: statuses[task.status] || "unknown",
      progress: Number(String(task.progress || "0").replace("%", "")),
      created_at: Number(task.created_at || 0),
    };
    const completedAt = Number(task.finished_at || task.updated_at || 0);
    if (completedAt > 0) output.completed_at = completedAt;
    if (task.status === "FAILURE") {
      output.error = { code: "video_generation_failed", message: "The video generation task failed." };
    }
    return output;
  },
};

// Muse accepts JSON attachments in order. Translate the gateway's media slots
// before submission. The host enforces the published multi-reference count.
function decodeMuseVideo(ctx) {
  if (ctx.body?.kind !== "json" || !ctx.body.value || Array.isArray(ctx.body.value)) {
    throw new Error("Muse video requires a JSON request");
  }
  const request = { ...ctx.body.value, model: ctx.model };
  const duration = Number(request.duration ?? request.seconds ?? 5);
  if (!Number.isInteger(duration) || duration < 1 || duration > 60 || (request.seconds !== undefined && Number(request.seconds) !== duration)) {
    throw new Error("Muse video duration must be an integer from 1 to 60 seconds");
  }
  request.duration = duration;
  delete request.seconds;
  if (request.size === "16:9" || (!request.size && request.aspect_ratio === "16:9")) request.size = "1280x720";
  if (request.size === "9:16" || (!request.size && request.aspect_ratio === "9:16")) request.size = "720x1280";

  const mediaFields = ["images", "image_urls", "image_with_roles", "image", "image_url", "reference_image", "input_reference"];
  const provided = mediaFields.filter(field => request[field] != null);
  if (provided.length > 1 || ["videos", "audios", "files", "video_urls", "audio_urls", "reference_videos", "reference_audios"].some(field => request[field] != null)) {
    throw new Error("Supply only one set of Muse video image inputs");
  }
  const field = provided[0];
  let references = [];
  if (field === "image_with_roles") {
    const frames = request[field];
    if (!Array.isArray(frames) || frames.length !== 1 || frames[0]?.role !== "first_frame") {
      throw new Error("Muse video first-frame mode requires one image");
    }
    references = [frames[0].url];
  } else if (field === "images" || field === "image_urls") {
    if (!Array.isArray(request[field])) throw new Error("Muse video references must be an array of images");
    references = request[field];
  } else if (field) {
    references = [request[field]];
  }
  references = references.map(reference => {
    const url = typeof reference === "string" ? reference : (reference?.url ?? reference?.image_url);
    if (typeof url !== "string" || !url.trim()) throw new Error("Muse video reference images must be non-empty");
    return url.trim();
  });
  let operation = request.operation;
  if (operation === undefined) {
    operation = "text_to_video";
    if (references.length === 1) operation = "first_frame";
    if (references.length > 1) operation = "video_multi_ref";
  }
  if (!["text_to_video", "first_frame", "video_multi_ref"].includes(operation)) {
    throw new Error("Muse video generation mode is unsupported");
  }
  if (operation === "first_frame") {
    if (references.length !== 1) throw new Error("Muse video first-frame mode requires one image");
  } else if (operation === "text_to_video" && references.length !== 0) {
    throw new Error("Muse text-to-video does not accept reference images");
  }
  for (const key of provided) delete request[key];
  delete request.operation;
  if (operation === "first_frame") request.image = references[0];
  if (operation === "video_multi_ref") request.images = references;
  return { kind: "submit", model: ctx.model, action: operation, requestBody: request };
}

// The economy route takes a single unroled image in the images array. Keep
// the prepared body on the same wire keys used by the submit and usage hooks.
function decodeSeedanceVideo(ctx) {
  const request = ctx.body?.value;
  if (ctx.body?.kind !== "json" || !request || typeof request !== "object" || Array.isArray(request)) {
    throw new Error("Seedance video requires a JSON object");
  }
  const fields = ["model", "group", "prompt", "operation", "images", "image_urls", "duration", "seconds", "aspect_ratio", "size", "resolution", "stream"];
  if (Object.keys(request).some(field => !fields.includes(field)) || (request.stream !== undefined && request.stream !== false)) {
    throw new Error("Unsupported field for this Seedance video route");
  }
  if (request.operation !== undefined && request.operation !== "video_multi_ref") {
    throw new Error("This Seedance video route requires single-image reference mode");
  }
  if (typeof request.prompt !== "string" || !request.prompt.trim()) throw new Error("field prompt is required");

  const imageFields = ["images", "image_urls"].filter(field => request[field] !== undefined);
  const images = request[imageFields[0]];
  if (imageFields.length !== 1 || !Array.isArray(images) || images.length !== 1 ||
      typeof images[0] !== "string" || !/^https?:\/\/[^\s/?#]+(?:[/?#][^\s]*)?$/i.test(images[0].trim())) {
    throw new Error("This Seedance video route requires exactly one image URL");
  }
  const durations = [request.duration, request.seconds].filter(value => value !== undefined)
    .map(value => typeof value === "string" && /^\d+$/.test(value) ? Number(value) : value);
  if (!durations.length || durations.some(value => !Number.isInteger(value) || value < 4 || value > 30 || value !== durations[0])) {
    throw new Error("Seedance video duration and seconds must specify the same integer from 4 to 30");
  }
  const aspectRatio = request.aspect_ratio === undefined ? request.size : request.aspect_ratio;
  if (aspectRatio !== "9:16" || (request.size !== undefined && request.size !== aspectRatio)) {
    throw new Error("This Seedance video route requires a 9:16 aspect ratio");
  }
  if (!["480p", "720p"].includes(request.resolution)) {
    throw new Error("Select a Seedance resolution group or specify resolution as 480p or 720p");
  }
  return {
    kind: "submit",
    model: ctx.model,
    action: "video_multi_ref",
    requestBody: {
      model: ctx.model,
      prompt: request.prompt,
      seconds: durations[0],
      resolution: request.resolution,
      aspect_ratio: aspectRatio,
      images: [images[0].trim()],
    },
  };
}

protocols.openai_video = {
  decodeRequest: function (ctx) {
    if (ctx.upstreamModel === SEEDANCE_VIDEO_DECODER) return decodeSeedanceVideo(ctx);
    if (ctx.model === MUSE_VIDEO_MODEL || ctx.upstreamModel === MUSE_VIDEO_MODEL) return decodeMuseVideo(ctx);
    if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
    if (ctx.body.kind === "json") {
      if (!ctx.body.value || Array.isArray(ctx.body.value)) throw new Error("JSON object required");
      const req = ctx.body.value;
      const seconds = req.seconds === undefined ? req.duration : req.seconds;
      if (seconds !== undefined && (!Number.isFinite(Number(seconds)) || Number(seconds) <= 0 || Number(seconds) > 3600))
        throw new Error("seconds must be between 1 and 3600");
      return {
        kind: "submit",
        model: ctx.model,
        action: req.input_reference || req.image ? "image_to_video" : "text_to_video",
        requestBody: Object.assign({}, req, { model: ctx.model }),
      };
    }
    const first = function (name) {
      const values = (ctx.body.fields || {})[name] || [];
      if (values.length > 1) throw new Error(name + " must be provided once");
      return values[0];
    };
    const req = {};
    const fields = ctx.body.fields || {};
    for (const name of Object.keys(fields)) {
      req[name] = first(name);
    }
    let hasInputReferenceFile = false;
    for (const file of ctx.body.files || []) {
      if (file.field !== "input_reference") throw new Error("unexpected file field: " + file.field);
      if (hasInputReferenceFile) throw new Error("input_reference must be provided once");
      hasInputReferenceFile = true;
    }
    if (req.metadata !== undefined) {
      let parsed;
      try {
        parsed = JSON.parse(req.metadata);
      } catch (e) {
        throw new Error("metadata must be a JSON object string");
      }
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("metadata must be a JSON object string");
      req.metadata = parsed;
    }
    if (req.seconds !== undefined) req.seconds = Number(req.seconds);
    else if (req.duration !== undefined) req.seconds = Number(req.duration);
    const seconds = req.seconds === undefined ? req.duration : req.seconds;
    if (seconds !== undefined && (!Number.isFinite(Number(seconds)) || Number(seconds) <= 0 || Number(seconds) > 3600))
      throw new Error("seconds must be between 1 and 3600");
    return {
      kind: "submit",
      model: ctx.model,
      action: hasInputReferenceFile || req.input_reference || req.image ? "image_to_video" : "text_to_video",
      requestBody: Object.assign({}, req, { model: ctx.model }),
    };
  },
  render: function (ctx, task) {
    if (task.data && typeof task.data === "object" && !Array.isArray(task.data)) return task.data;
    return legacyRenderers.openai_video(task);
  },
};
