/**
 * HTTP layer for the omnimux CLI.
 *
 * Ported from .agents/skills/newapi/scripts/http.js (fetchJson, authHeaders,
 * joinUrl) and the request/masking flow of api.js.
 *
 * Never logs the Authorization header or any secret. Errors are typed
 * (network / auth / api) — subtype is never inferred from free-text message.
 */

const { maskSensitiveJson, isSensitivePath, sanitize } = require("./mask");
const { ApiError, AuthError, NetworkError } = require("./errors");

function authHeaders(accessToken, userId) {
  return {
    Authorization: `Bearer ${accessToken}`,
    "New-Api-User": String(userId),
  };
}

function joinUrl(base, urlPath) {
  const b = String(base || "").replace(/\/+$/, "");
  const p = String(urlPath || "");
  if (!p) return b;
  return p.startsWith("/") ? b + p : b + "/" + p;
}

/**
 * Extract a request id from response headers or JSON body when present.
 * @param {Response|null} res
 * @param {any} json
 */
function extractRequestId(res, json) {
  if (res && typeof res.headers && res.headers.get) {
    const h =
      res.headers.get("x-oneapi-request-id") ||
      res.headers.get("x-request-id") ||
      res.headers.get("x-trace-id");
    if (h) return String(h);
  }
  if (json && typeof json === "object") {
    if (json.request_id) return String(json.request_id);
    if (json.requestId) return String(json.requestId);
  }
  return undefined;
}

/**
 * Classify a failed HTTP response into a typed error.
 * Precedence: 401 → auth; 429 → rate_limit; 404 → not_found; else upstream.
 * @param {number} status
 * @param {any} json
 * @param {string} [requestId]
 */
function classifyHttpFailure(status, json, requestId) {
  const rawMsg =
    (json && typeof json.message === "string" && json.message) ||
    (json && typeof json.error === "string" && json.error) ||
    `HTTP ${status}`;
  const message = sanitize(String(rawMsg));

  if (status === 401) {
    return new AuthError(message || "authentication failed", {
      subtype: "token_invalid",
      requestId,
      hint: "re-authenticate then retry",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  const creationCode = json && typeof json.code === "string" ? json.code : "";
  if (status === 403 && (creationCode.startsWith("influencer_") || creationCode === "ACCESS_TOKEN_SCOPE_DENIED")) {
    return new AuthError(message, { subtype: "scope_required", requestId, hint: "sign in again and explicitly authorize character and video creation", recovery: { command: "omnimux", args: ["login", "--no-wait"] } });
  }
  const creationCodes = new Set([
    "influencer_creation_disabled", "creation_request_conflict", "creation_request_expired",
    "creation_outcome_unknown", "creation_request_unauthorized", "invalid_request", "invalid_selection",
    "invalid_reference", "invalid_video", "media_expired", "reference_route_unavailable", "task_not_ready",
    "artifact_unavailable", "unsupported_media", "unsupported_aspect_ratio", "unsupported_resolution",
    "creation_route_unavailable", "generation_route_unavailable", "invalid_reference_image", "invalid_parent_character", "creation_material_unavailable",
    "artifact_projection_unavailable", "invalid_source_character", "invalid_source_video", "generation_queue_unavailable",
    "task_read_unavailable", "media_inspection_unavailable", "invalid_reference_video", "invalid_video_metadata",
  ]);
  const identity = {
    operationId: json && typeof json.operation_id === "string" && /^[A-Za-z0-9._:-]{8,128}$/.test(json.operation_id) ? json.operation_id : undefined,
    taskId: json && typeof json.task_id === "string" && /^[A-Za-z0-9._:-]{1,128}$/.test(json.task_id) ? json.task_id : undefined,
  };
  if (creationCodes.has(creationCode)) {
    return new ApiError(message, { subtype: creationCode, status, requestId, ...identity, retryable: ["generation_queue_unavailable", "task_read_unavailable", "artifact_projection_unavailable"].includes(creationCode) });
  }
  if (status === 429) {
    return new ApiError(message || "rate limited", {
      subtype: "rate_limit",
      status,
      retryable: true,
      requestId,
      hint: "wait and retry with backoff",
    });
  }
  if (status === 404) {
    return new ApiError(message || "not found", {
      subtype: "not_found",
      status,
      requestId,
    });
  }
  // Quota signals when server uses a stable code field (never scrape message).
  const code = json && (json.code != null ? String(json.code) : "");
  if (status === 402 || code === "quota_exceeded" || code === "insufficient_quota") {
    return new ApiError(message || "quota exceeded", {
      subtype: "quota_exceeded",
      status,
      requestId,
    });
  }
  if (code === "SOCIAL_SEAT_QUOTA") {
    return new ApiError("社媒绑定席位不足或余额不够", {
      subtype: "quota_exceeded",
      status,
      requestId,
    });
  }
  if (code === "SOCIAL_SEAT_CAP") {
    return new ApiError("社媒绑定席位已达上限，请先解绑一个账号", {
      subtype: "upstream",
      status,
      requestId,
    });
  }
  if (code === "SOCIAL_SEAT_EXPIRED") {
    return new ApiError("社媒绑定席位已过期，请续费或解绑", {
      subtype: "upstream",
      status,
      requestId,
    });
  }
  return new ApiError(message, {
    subtype: "upstream",
    status,
    requestId,
    retryable: status >= 500,
  });
}

/**
 * Classify a fetch/transport failure.
 * @param {unknown} err
 * @param {string} urlPath
 */
function classifyTransportFailure(err, urlPath) {
  const name = err && /** @type {any} */ (err).name;
  if (name === "AbortError") {
    return new NetworkError(`request timed out while calling ${urlPath}`, {
      subtype: "timeout",
      hint: "retry with backoff; increase timeout if the server is slow",
      cause: err instanceof Error ? err : undefined,
    });
  }
  const msg = err instanceof Error ? err.message : String(err || "network error");
  const lower = msg.toLowerCase();
  let subtype = "transport";
  if (lower.includes("enotfound") || lower.includes("getaddrinfo") || lower.includes("dns")) {
    subtype = "dns";
  }
  return new NetworkError(`network error while calling ${urlPath}`, {
    subtype,
    hint: "check connectivity and base_url, then retry",
    cause: err instanceof Error ? err : undefined,
  });
}

async function fetchJson(url, { method = "GET", headers = {}, body = null, timeoutMs = 20000, signal, disableKeepAlive = false } = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  const abort = () => controller.abort();
  if (signal?.aborted) abort();
  signal?.addEventListener("abort", abort, { once: true });
  try {
    const opts = { method, headers, signal: controller.signal };
    if (disableKeepAlive) {
      // Bun may replay a pooled request after a reset; creation permits one send only.
      opts.keepalive = false;
      opts.redirect = "manual";
    }
    if (body != null) {
      opts.headers = { ...headers, "Content-Type": "application/json" };
      opts.body = typeof body === "string" ? body : JSON.stringify(body);
    }
    const res = await fetch(url, opts);
    const text = await res.text();
    let json = null;
    try {
      json = text ? JSON.parse(text) : null;
    } catch {
      json = null;
    }
    return {
      ok: res.status >= 200 && res.status < 300,
      status: res.status,
      text,
      json,
      headers: res.headers,
    };
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}

/**
 * Call a user-level API endpoint.
 *
 * @param {{ baseUrl: string, accessToken?: string, userId?: string }} auth
 * @param {string} method
 * @param {string} urlPath path starting with /api/...
 * @param {object|null} [body]
 * @param {object|null} [extraHeaders]
 * @returns {Promise<any>}
 */
async function apiRequest(auth, method, urlPath, body = null, extraHeaders = null, requestOptions = {}) {
  const headers = {};
  if (auth.accessToken) {
    Object.assign(headers, authHeaders(auth.accessToken, auth.userId || ""));
  }
  if (extraHeaders && typeof extraHeaders === "object") {
    for (const [k, v] of Object.entries(extraHeaders)) {
      if (v != null && v !== "") headers[k] = String(v);
    }
  }

  let res;
  try {
    res = await fetchJson(joinUrl(auth.baseUrl, urlPath), { method, headers, body, timeoutMs: requestOptions.timeoutMs || 20000, signal: requestOptions.signal, disableKeepAlive: Boolean(requestOptions.noRetry) });
  } catch (err) {
    throw classifyTransportFailure(err, urlPath);
  }

  const requestId = extractRequestId({ headers: res.headers }, res.json);

  if (!res.ok) {
    throw classifyHttpFailure(res.status, res.json, requestId);
  }

  let data = res.json;
  if (isSensitivePath(urlPath)) {
    if (data === null) {
      throw new ApiError("response omitted because masking failed", {
        subtype: "upstream",
        status: res.status,
        requestId,
      });
    }
    data = maskSensitiveJson(data);
  }

  if (data && data.success === false) {
    // Stable business codes retain their classification even with HTTP 200.
    throw classifyHttpFailure(res.status, data, requestId);
  }
  return data;
}

/**
 * Fetch the full `sk-...` key for a token. In-memory only — callers must
 * pipe it to the clipboard / a file / a child process, never to stdout.
 */
async function fetchTokenKey(auth, tokenId) {
  let res;
  try {
    res = await fetchJson(joinUrl(auth.baseUrl, `/api/token/${tokenId}/key`), {
      method: "POST",
      headers: authHeaders(auth.accessToken, auth.userId || ""),
    });
  } catch (err) {
    throw classifyTransportFailure(err, `/api/token/${tokenId}/key`);
  }

  const requestId = extractRequestId({ headers: res.headers }, res.json);
  if (!res.ok) {
    throw classifyHttpFailure(res.status, res.json, requestId);
  }

  const bodyJson = res.json;
  if (!bodyJson || (!bodyJson.success && !bodyJson.data)) {
    throw new ApiError(sanitize((bodyJson && bodyJson.message) || "Unknown API error"), {
      subtype: "upstream",
      status: res.status,
      requestId,
    });
  }
  const rawKey = bodyJson.data && bodyJson.data.key;
  if (!rawKey) {
    throw new ApiError("API response did not contain a key", {
      subtype: "upstream",
      status: res.status,
      requestId,
    });
  }
  const s = String(rawKey);
  return s.startsWith("sk-") ? s : "sk-" + s;
}

module.exports = {
  ApiError,
  AuthError,
  NetworkError,
  authHeaders,
  joinUrl,
  fetchJson,
  apiRequest,
  fetchTokenKey,
  classifyHttpFailure,
  classifyTransportFailure,
  extractRequestId,
};
