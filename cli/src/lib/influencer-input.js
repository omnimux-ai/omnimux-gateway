/** Local-only input normalization. Never reads credentials, uploads, or probes media. */
const fs = require("fs");
const path = require("path");
const { UsageError } = require("./errors");
const { guessContentType } = require("./upload");
const API_ROOT = "/api/influencer/v1";
const REQUEST_ID = /^[A-Za-z0-9._:-]{8,128}$/;
const LIMITS = { image: 5 * 1024 * 1024, video: 100 * 1024 * 1024 };
const CREATE_FIELDS = {
  character: ["request_id", "tier", "selection", "brief", "image_url"],
  video: ["request_id", "mode", "character_task_id", "image_urls", "video_url", "video_task_id", "resolution", "aspect_ratio", "prompt"],
  multiview: ["request_id"],
};
const FLAG_FIELDS = {
  "request-id": "request_id", type: "tier", brief: "brief", selection: "selection",
  "reference-image": "image_url", character: "character_task_id", image: "image_urls",
  video: "video_url", "video-task": "video_task_id", mode: "mode", resolution: "resolution",
  "aspect-ratio": "aspect_ratio", prompt: "prompt", input: "input", page: "page",
  "page-size": "page_size", kind: "kind", status: "status", output: "output", timeout: "timeout",
};
function invalid(message, param) { return new UsageError(message, { subtype: "invalid_argument", param }); }
function parseFlags(args, globals = {}) {
  const flags = {}, positional = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith("--")) { positional.push(arg); continue; }
    const eq = arg.indexOf("=");
    const name = arg.slice(2, eq < 0 ? undefined : eq);
    if (name === "overwrite") { if (eq >= 0) throw invalid("--overwrite takes no value", arg); flags.overwrite = true; continue; }
    const field = FLAG_FIELDS[name];
    if (!field) throw invalid(`unknown influencer flag: --${name}`, `--${name}`);
    const value = eq >= 0 ? arg.slice(eq + 1) : args[++i];
    if (value === undefined || value === "" || value.startsWith("--")) throw invalid(`--${name} requires a value`, `--${name}`);
    if (field === "image_urls") (flags[field] ||= []).push(value);
    else {
      if (Object.hasOwn(flags, field)) throw invalid(`--${name} must be supplied once`, `--${name}`);
      flags[field] = value;
    }
  }
  // The existing root parser owns these flags. Preserve them deliberately.
  for (const [root, local] of [["mode", "mode"], ["pageSize", "page_size"], ["timeout", "timeout"]]) {
    if (globals[root] !== undefined) {
      if (Object.hasOwn(flags, local)) throw invalid(`duplicate --${local.replaceAll("_", "-")}`);
      flags[local] = globals[root];
    }
  }
  for (const key of ["limit", "model", "group", "quota", "scene", "query", "category", "language", "resume"]) {
    if (globals[key] !== undefined) throw invalid(`--${key} is not supported for influencer`, `--${key}`);
  }
  return { flags, positional };
}
function onlyFlags(flags, allowed) {
  for (const field of Object.keys(flags)) if (!allowed.includes(field)) throw invalid(`unsupported field: ${field}`, field);
}
function requireId(value, label = "task_id") {
  if (typeof value !== "string" || !/^[A-Za-z0-9._:-]{1,128}$/.test(value)) throw invalid(`invalid ${label}`, label);
  return value;
}
function requestId(value) {
  if (typeof value !== "string" || !REQUEST_ID.test(value)) throw invalid("request_id must be 8-128 ASCII letters, digits, . _ : or -", "--request-id");
  return value;
}
function text(value, field, max) {
  if (typeof value !== "string" || [...value].length > max) throw invalid(`${field} must be a string of at most ${max} characters`, field);
  return value;
}
function material(value, kind) {
  if (typeof value !== "string" || !value.trim()) throw invalid(`invalid ${kind} reference`);
  if (/^https?:\/\//i.test(value)) {
    let url;
    try { url = new URL(value); } catch { throw invalid(`invalid ${kind} URL`); }
    if (url.protocol !== "https:" || url.username || url.password || !url.hostname || url.hash) throw invalid(`${kind} URL must use HTTPS without credentials or fragment`);
    return url.href;
  }
  if (/^[a-z][a-z0-9+.-]*:/i.test(value) && !/^[A-Za-z]:[\\/]/.test(value)) throw invalid(`${kind} reference must be a local file or HTTPS URL`);
  return path.resolve(value);
}
function validateLocalFile(file, kind) {
  let stat;
  try { stat = fs.statSync(file); } catch { throw invalid(`${kind} file is unavailable`, kind); }
  if (!stat.isFile() || stat.size === 0) throw invalid(`${kind} must be a nonempty regular file`, kind);
  if (stat.size > LIMITS[kind]) throw invalid(`${kind} exceeds ${LIMITS[kind] / 1024 / 1024} MiB`, kind);
  const mime = guessContentType(file);
  const valid = kind === "image" ? ["image/png", "image/jpeg", "image/webp"] : ["video/mp4", "video/quicktime"];
  if (!valid.includes(mime)) throw invalid(`unsupported ${kind} file type`, kind);
  return { bytes: stat.size, mime_type: mime };
}
function materials(kind, body) {
  if (kind === "character") return body.image_url ? [{ field: "image_url", value: body.image_url, kind: "image" }] : [];
  if (kind !== "video") return [];
  const refs = (body.image_urls || []).map((value, index) => ({ field: "image_urls", index, value, kind: "image" }));
  if (body.video_url) refs.push({ field: "video_url", value: body.video_url, kind: "video" });
  return refs;
}
function isRemote(value) { return value.startsWith("https://"); }
function createInput(kind, flags) {
  let input = {};
  if (flags.input) {
    try { input = JSON.parse(fs.readFileSync(path.resolve(flags.input), "utf8")); } catch { throw invalid("--input must name a readable JSON object", "--input"); }
    if (!input || Array.isArray(input) || typeof input !== "object") throw invalid("--input must contain an object", "--input");
    for (const [field, value] of Object.entries(flags)) {
      if (field === "input") continue;
      if (Object.hasOwn(input, field)) throw invalid(`explicit ${field} overlaps the input file`, field);
      input[field] = value;
    }
  } else input = { ...flags };
  onlyFlags(input, CREATE_FIELDS[kind]);
  if (input.request_id !== undefined) requestId(input.request_id);
  if (kind === "character") {
    input.tier ??= "normal";
    if (!["normal", "freak", "total", "cats", "dogs"].includes(input.tier)) throw invalid("unsupported character type", "--type");
    if (typeof input.selection === "string") {
      try { input.selection = JSON.parse(input.selection); } catch { throw invalid("--selection must be a JSON category-to-option-array object", "--selection"); }
    }
    if (input.selection !== undefined) {
      const sel = input.selection;
      if (!sel || Array.isArray(sel) || typeof sel !== "object" || Object.values(sel).some((v) => !Array.isArray(v) || v.some((id) => typeof id !== "string" || !id))) throw invalid("selection must map categories to option ID arrays", "--selection");
    }
    if (input.brief !== undefined) text(input.brief, "brief", 4000);
    if (input.image_url !== undefined) input.image_url = material(input.image_url, "image");
    if (!["cats", "dogs"].includes(input.tier) && !(input.brief || "").trim() && !input.image_url && !Object.values(input.selection || {}).some((v) => v.length)) throw invalid("provide a brief, selection, or reference image");
  }
  if (kind === "video") {
    input.mode ??= "motion"; input.resolution ??= "480p";
    if (!["motion", "replace"].includes(input.mode)) throw invalid("mode must be motion or replace", "--mode");
    if (!["480p", "720p"].includes(input.resolution)) throw invalid("resolution must be 480p or 720p", "--resolution");
    if (input.aspect_ratio !== undefined && !(input.resolution === "480p" ? ["16:9", "9:16", "3:4"] : ["16:9", "9:16"]).includes(input.aspect_ratio)) throw invalid("unsupported resolution/aspect ratio pair", "--aspect-ratio");
    if (input.character_task_id !== undefined) requireId(input.character_task_id);
    if (input.video_task_id !== undefined) requireId(input.video_task_id);
    if (input.video_url !== undefined && input.video_task_id !== undefined) throw invalid("--video and --video-task are exclusive");
    if (!input.video_url && !input.video_task_id) throw invalid("provide --video or --video-task");
    if (input.video_url !== undefined) input.video_url = material(input.video_url, "video");
    if (input.image_urls !== undefined && !Array.isArray(input.image_urls)) throw invalid("image_urls must be an ordered array");
    input.image_urls = (input.image_urls || []).map((v) => material(v, "image"));
    const count = input.image_urls.length + (input.character_task_id ? 1 : 0);
    if (count < 1 || count > 30) throw invalid("video requires 1-30 total character and reference images", "--image");
    if (input.prompt !== undefined) text(input.prompt, "prompt", 4000);
  }
  return input;
}
function positiveInt(value, field, fallback, max) {
  if (value === undefined) return fallback;
  const n = Number(value);
  if (!Number.isInteger(n) || n < 1 || n > max) throw invalid(`${field} must be an integer between 1 and ${max}`, field);
  return n;
}
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === "object") return Object.fromEntries(Object.keys(value).sort().map((k) => [k, canonical(value[k])]));
  return value;
}
module.exports = { API_ROOT, LIMITS, parseFlags, onlyFlags, requireId, requestId, createInput, materials, isRemote, validateLocalFile, positiveInt, canonical, invalid };
