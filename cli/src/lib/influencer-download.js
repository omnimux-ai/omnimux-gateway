/** Bounded media download; API credentials never accompany storage or redirects. */
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { ApiError, NetworkError, UsageError } = require("./errors");
const { request } = require("./influencer");
const MAX_BYTES = 1024 * 1024 * 1024;
function mediaType(bytes) {
  if (bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))) return "image/png";
  if (bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255) return "image/jpeg";
  if (["GIF87a", "GIF89a"].includes(bytes.subarray(0, 6).toString())) return "image/gif";
  if (bytes.subarray(0, 4).toString() === "RIFF" && bytes.subarray(8, 12).toString() === "WEBP") return "image/webp";
  if (bytes.subarray(4, 8).toString() === "ftyp") return "video/mp4";
  if (bytes.subarray(0, 4).equals(Buffer.from([26, 69, 223, 163]))) return "video/webm";
  return "";
}
function destination(raw, base) {
  let url;
  try { url = new URL(raw, base); } catch { throw new ApiError("artifact address is invalid", { subtype: "artifact_unavailable" }); }
  if (url.username || url.password || url.hash) throw new ApiError("artifact address is unsafe", { subtype: "artifact_unavailable" });
  const baseURL = new URL(base);
  const ownedAPI = url.origin === baseURL.origin && /\/(?:api\/influencer\/v1\/tasks|api\/task|v1\/tasks)\/[^/]+\/artifacts\/[^/]+\/content$/.test(url.pathname);
  if (!ownedAPI && url.protocol !== "https:") throw new ApiError("artifact storage must use HTTPS", { subtype: "artifact_unavailable" });
  if (ownedAPI && !["http:", "https:"].includes(url.protocol)) throw new ApiError("artifact address is unsafe", { subtype: "artifact_unavailable" });
  return { url, ownedAPI };
}
async function download(ctx, taskId, output, overwrite = false) {
  const target = path.resolve(output);
  const recover = { command: "omnimux", args: ["influencer", "task", "download", taskId, "--output", output, ...(overwrite ? ["--overwrite"] : [])] };
  if (!overwrite && fs.existsSync(target)) throw new UsageError("output already exists; choose another path or explicitly use --overwrite", { subtype: "output_exists", param: "--output" });
  let parent;
  try { parent = fs.statSync(path.dirname(target)); } catch {}
  if (!parent?.isDirectory()) throw new UsageError("output directory does not exist", { param: "--output" });
  const data = await request(ctx, "GET", `/tasks/${encodeURIComponent(taskId)}/artifacts`);
  const artifacts = data && data.artifacts;
  if (!Array.isArray(artifacts) || artifacts.length === 0) throw new ApiError("task has no downloadable artifact", { subtype: "artifact_unavailable", taskId });
  if (artifacts.length !== 1) throw new ApiError("task has multiple artifacts; a single output cannot select one safely", { subtype: "artifact_selection_required", taskId });
  const artifact = artifacts[0];
  if (!artifact.content_url || !["image", "video"].includes(artifact.type)) throw new ApiError("task artifact is not supported", { subtype: "artifact_unavailable", taskId });
  const first = destination(artifact.content_url, ctx.auth.baseUrl);
  let url = first.url;
  if (first.ownedAPI && !first.url.searchParams.get("access")) throw new ApiError("artifact API URL is missing its download authorization", { subtype: "artifact_unavailable", taskId });
  let headers = {};
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 300000);
  const interrupt = () => controller.abort();
  process.once("SIGINT", interrupt); process.once("SIGTERM", interrupt);
  const temp = path.join(path.dirname(target), `.${path.basename(target)}.omnimux-${crypto.randomUUID()}.part`);
  let file, bytes = 0, prefix = Buffer.alloc(0);
  try {
    let response;
    for (let hop = 0; hop <= 5; hop++) {
      response = await fetch(url.href, { headers, redirect: "manual", signal: controller.signal });
      if (![301, 302, 303, 307, 308].includes(response.status)) break;
      const location = response.headers.get("location");
      await response.body?.cancel();
      if (!location || hop === 5) throw new ApiError("artifact redirect is invalid", { subtype: "artifact_unavailable", taskId });
      const next = destination(location, url.href);
      if (next.url.protocol !== "https:") throw new ApiError("artifact redirect must use HTTPS", { subtype: "artifact_unavailable", taskId });
      url = next.url; headers = {}; // Never forward account credentials through a redirect.
    }
    if (!response.ok) {
      await response.body?.cancel();
      throw new ApiError(response.status === 403 || response.status === 404 || response.status === 410 ? "artifact link is unavailable; retrieve the original task again" : "artifact download failed", { subtype: [403, 404, 410].includes(response.status) ? "artifact_expired" : "download_failed", status: response.status, taskId, recovery: recover, retryable: true });
    }
    // This command does not request ranges or implement resume. A self-consistent
    // fragment length and valid container prefix do not prove a complete artifact.
    if (response.status !== 200 || response.headers.has("content-range")) {
      await response.body?.cancel();
      throw new ApiError("artifact response is partial or incomplete", { subtype: "artifact_invalid", status: response.status, taskId });
    }
    const contentType = (response.headers.get("content-type") || "").split(";")[0].trim().toLowerCase();
    const expectedType = artifact.type + "/";
    if (!contentType.startsWith(expectedType)) {
      await response.body?.cancel();
      throw new ApiError("artifact content type does not match the requested media", { subtype: "artifact_invalid", taskId });
    }
    const rawLength = response.headers.get("content-length");
    const length = rawLength === null ? null : Number(rawLength);
    if (length !== null && (!Number.isSafeInteger(length) || length < 1 || length > MAX_BYTES)) {
      await response.body?.cancel();
      throw new ApiError("artifact length is outside the supported limit", { subtype: "artifact_invalid", taskId });
    }
    if (!response.body) throw new ApiError("artifact response has no media", { subtype: "artifact_invalid", taskId });
    file = await fs.promises.open(temp, "wx", 0o600);
    for await (const part of response.body) {
      const buffer = Buffer.from(part);
      bytes += buffer.length;
      if (bytes > MAX_BYTES) throw new ApiError("artifact exceeds download size limit", { subtype: "artifact_invalid", taskId });
      if (prefix.length < 32) prefix = Buffer.concat([prefix, buffer.subarray(0, 32 - prefix.length)]);
      let offset = 0;
      while (offset < buffer.length) {
        const written = await file.write(buffer, offset, buffer.length - offset);
        if (!written.bytesWritten) throw new Error("short file write");
        offset += written.bytesWritten;
      }
    }
    if (!bytes || (length !== null && bytes !== length) || !mediaType(prefix).startsWith(expectedType)) throw new ApiError("artifact file is incomplete or invalid", { subtype: "artifact_invalid", taskId });
    await file.sync(); await file.close(); file = null;
    if (overwrite) await fs.promises.rename(temp, target);
    else {
      try { await fs.promises.link(temp, target); }
      catch (err) { if (err.code === "EEXIST") throw new UsageError("output already exists", { subtype: "output_exists", param: "--output" }); throw err; }
      await fs.promises.unlink(temp);
    }
    return { task_id: taskId, output: target, bytes, mime_type: contentType, artifact_key: artifact.key };
  } catch (err) {
    if (err.type) { err.taskId = taskId; err.recovery ||= recover; throw err; }
    const failure = new NetworkError("artifact download was interrupted; retry the original task download", { subtype: err.name === "AbortError" ? "timeout" : "download_interrupted" }).withRecovery(recover.command, recover.args);
    failure.taskId = taskId;
    throw failure;
  } finally {
    clearTimeout(timer);
    process.removeListener("SIGINT", interrupt); process.removeListener("SIGTERM", interrupt);
    if (file) await file.close().catch(() => {});
    await fs.promises.unlink(temp).catch(() => {});
  }
}
module.exports = { download, mediaType, MAX_BYTES };
