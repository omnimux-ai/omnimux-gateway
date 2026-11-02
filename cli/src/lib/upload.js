/**
 * Binary PUT for presigned media uploads (no JSON, no auth header on the storage URL).
 */

const { ApiError } = require("./http");

const DEFAULT_MAX_BYTES = 512 * 1024 * 1024; // 512 MiB client guard

/**
 * @param {string} uploadUrl
 * @param {Buffer|Uint8Array} body
 * @param {string} contentType
 * @param {{ timeoutMs?: number }} [opts]
 */
async function putBinary(uploadUrl, body, contentType, opts = {}) {
  const timeoutMs = opts.timeoutMs || 300000;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(uploadUrl, {
      method: "PUT",
      headers: {
        "Content-Type": contentType || "application/octet-stream",
      },
      body,
      signal: controller.signal,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new ApiError(
        `media upload failed HTTP ${res.status}${text ? ": " + text.slice(0, 200) : ""}`,
        res.status
      );
    }
    return { status: res.status, bytes: body.length };
  } catch (err) {
    if (err instanceof ApiError) throw err;
    const { NetworkError } = require("./errors");
    if (err && err.name === "AbortError") {
      throw new NetworkError("upload timed out", { subtype: "timeout", cause: err });
    }
    throw new NetworkError("upload network error", {
      subtype: "transport",
      cause: err instanceof Error ? err : undefined,
    });
  } finally {
    clearTimeout(timer);
  }
}

function guessContentType(filename) {
  const name = String(filename || "").toLowerCase();
  if (name.endsWith(".png")) return "image/png";
  if (name.endsWith(".jpg") || name.endsWith(".jpeg")) return "image/jpeg";
  if (name.endsWith(".gif")) return "image/gif";
  if (name.endsWith(".webp")) return "image/webp";
  if (name.endsWith(".mp4")) return "video/mp4";
  if (name.endsWith(".mov")) return "video/quicktime";
  if (name.endsWith(".webm")) return "video/webm";
  if (name.endsWith(".m4v")) return "video/x-m4v";
  if (name.endsWith(".mp3")) return "audio/mpeg";
  if (name.endsWith(".m4a")) return "audio/mp4";
  if (name.endsWith(".aac")) return "audio/aac";
  if (name.endsWith(".wav")) return "audio/wav";
  if (name.endsWith(".pdf")) return "application/pdf";
  return "";
}

/**
 * Upload a local file to the OmniMux gateway file service (/api/v1/files/upload/stream).
 * The gateway stores the file in Cloudflare R2 (omnimux-files) with a 72-hour protection policy.
 *
 * @param {{ baseUrl: string, accessToken?: string, userId?: string }} auth
 * @param {string} filePath
 * @param {string} [uploadPath="photos"]
 * @param {{ timeoutMs?: number }} [opts]
 * @returns {Promise<{ file_id: string, file_url: string, download_url: string, file_name: string, file_size: number, mime_type: string }>}
 */
async function uploadGatewayFile(auth, filePath, uploadPath = "photos", opts = {}) {
  const fs = require("fs");
  const path = require("path");
  const { UsageError } = require("./errors");
  const {
    authHeaders,
    joinUrl,
    extractRequestId,
    classifyHttpFailure,
    classifyTransportFailure,
  } = require("./http");

  if (!fs.existsSync(filePath)) {
    throw new UsageError(`file not found: ${filePath}`);
  }
  const stat = fs.statSync(filePath);
  if (!stat.isFile()) {
    throw new UsageError(`not a file: ${filePath}`);
  }
  const maxBytes = opts.maxBytes || DEFAULT_MAX_BYTES;
  if (stat.size > maxBytes) {
    throw new UsageError(
      `file exceeds maximum size limit of ${maxBytes} bytes`
    );
  }

  const fileBytes = fs.readFileSync(filePath);
  if (opts.expectedSha256 && require("crypto").createHash("sha256").update(fileBytes).digest("hex") !== opts.expectedSha256) {
    throw new UsageError("local material changed before upload", { subtype: "creation_request_conflict" });
  }
  const fileName = path.basename(filePath);
  const contentType = guessContentType(fileName) || "application/octet-stream";

  const formData = new FormData();
  formData.append(
    "file",
    new Blob([fileBytes], { type: contentType }),
    fileName
  );
  formData.append("upload_path", uploadPath);
  formData.append("file_name", fileName);

  const urlPath = "/api/v1/files/upload/stream";
  const url = joinUrl(auth.baseUrl, urlPath);
  const headers = auth.accessToken
    ? authHeaders(auth.accessToken, auth.userId || "")
    : {};

  const timeoutMs = opts.timeoutMs || 300000;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  let res;
  try {
    res = await fetch(url, {
      method: "POST",
      headers,
      body: formData,
      signal: controller.signal,
    });
  } catch (err) {
    throw classifyTransportFailure(err, urlPath);
  } finally {
    clearTimeout(timer);
  }

  const text = await res.text();
  let json = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = null;
  }

  const requestId = extractRequestId({ headers: res.headers }, json);
  if (!res.ok) {
    throw classifyHttpFailure(res.status, json, requestId);
  }

  if (json && json.success === false) throw classifyHttpFailure(res.status, json, requestId);
  if (json && json.data) {
    return json.data;
  }
  return json;
}

module.exports = {
  putBinary,
  guessContentType,
  uploadGatewayFile,
  DEFAULT_MAX_BYTES,
};
