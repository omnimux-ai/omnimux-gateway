/**
 * omnimux login — device-code browser login (RFC 8628-style).
 *
 *   omnimux login [--no-open]
 *   omnimux login --no-wait [--no-open]   # Agent: return flow_id, no poll
 *   omnimux login --resume <flow_id>      # Agent: complete pending flow
 *   omnimux login status [--verify]       # Machine-readable status
 *
 * Server device_code is never printed. Resume uses opaque flow_id only.
 * stdout is JSON envelope only; never prints the access token value.
 */

const { setStoredToken, accessTokenMeta, resolveAccessToken } = require("../lib/secrets");
const {
  loadConfig,
  saveConfig,
  resolveBaseUrl,
  resolveUserId,
  isBlank,
} = require("../lib/config");
const { openUrl } = require("../lib/open-url");
const { fetchJson, joinUrl, apiRequest } = require("../lib/http");
const { UsageError, AuthError, NetworkError, ApiError } = require("../lib/errors");
const { sanitize } = require("../lib/mask");
const {
  savePendingFlow,
  loadPendingFlow,
  deletePendingFlow,
} = require("../lib/login-flow");

const GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code";
const MAX_SLOW_DOWN_MS = 30000;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function hasFlag(args, flags, name) {
  if (name === "no-open" && flags && flags.noOpen) return true;
  if (name === "no-wait" && flags && flags.noWait) return true;
  if (name === "verify" && flags && flags.verify) return true;
  const long = `--${name}`;
  return (args || []).includes(long);
}

function flagValue(args, name) {
  const long = `--${name}`;
  const eq = `--${name}=`;
  for (let i = 0; i < (args || []).length; i++) {
    const a = args[i];
    if (a === long && args[i + 1] && !String(args[i + 1]).startsWith("-")) {
      return args[i + 1];
    }
    if (a.startsWith(eq)) return a.slice(eq.length);
  }
  return "";
}

async function postJson(baseUrl, path, body) {
  const url = joinUrl(baseUrl, path);
  return fetchJson(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body,
  });
}

/**
 * Poll until success or terminal failure. Mutates nothing until success.
 * @returns {Promise<{ userId: string, username: string }>}
 */
async function pollUntilDone(baseUrl, deviceCode, expiresAt, startIntervalMs) {
  let intervalMs = startIntervalMs;
  while (Date.now() < expiresAt) {
    await sleep(intervalMs);
    let pollRes;
    try {
      pollRes = await postJson(baseUrl, "/api/user/device/token", {
        device_code: deviceCode,
        grant_type: GRANT_TYPE,
      });
    } catch (err) {
      throw new NetworkError("network error while polling device token", {
        subtype: "transport",
        cause: err instanceof Error ? err : undefined,
      });
    }

    if (!pollRes.ok && pollRes.status === 0) {
      throw new NetworkError("network error while polling device token", {
        subtype: "transport",
      });
    }

    const body = pollRes.json || {};
    if (body.success && body.data && body.data.access_token) {
      const pat = String(body.data.access_token);
      const userId = body.data.user_id != null ? String(body.data.user_id) : "";
      const username = body.data.username || "";
      setStoredToken(pat);
      if (userId) {
        const cfg = loadConfig();
        cfg.user_id = userId;
        saveConfig(cfg);
      }
      return { userId, username };
    }

    const code = body.code || "";
    if (code === "authorization_pending") {
      if (body.interval) {
        intervalMs = Math.max(1000, Number(body.interval) * 1000);
      }
      continue;
    }
    if (code === "slow_down") {
      const next = body.interval ? Number(body.interval) * 1000 : intervalMs + 5000;
      intervalMs = Math.min(MAX_SLOW_DOWN_MS, Math.max(intervalMs, next));
      continue;
    }
    if (code === "access_denied") {
      throw new AuthError("authorization denied in the browser", {
        subtype: "login_denied",
        hint: "start a new login if the user intended to authorize",
        recovery: { command: "omnimux", args: ["login", "--no-wait"] },
      });
    }
    if (code === "expired_token" || code === "invalid_grant") {
      throw new AuthError(sanitize(String(body.message || "device code expired or invalid")), {
        subtype: "login_expired",
        recovery: { command: "omnimux", args: ["login", "--no-wait"] },
      });
    }
    if (pollRes.status >= 400 && pollRes.status < 500) {
      throw new AuthError(
        sanitize(String(body.message || `device token poll failed (HTTP ${pollRes.status})`)),
        { subtype: "login_denied" }
      );
    }
  }
  throw new AuthError("timed out waiting for browser authorization", {
    subtype: "login_expired",
    recovery: { command: "omnimux", args: ["login", "--no-wait"] },
  });
}

async function startDeviceCode(baseUrl) {
  let codeRes;
  try {
    codeRes = await postJson(baseUrl, "/api/user/device/code", {
      client_name: "omnimux-cli",
    });
  } catch (err) {
    throw new NetworkError("network error while requesting device code", {
      subtype: "transport",
      cause: err instanceof Error ? err : undefined,
    });
  }
  if (!codeRes.ok || !codeRes.json || !codeRes.json.success || !codeRes.json.data) {
    const msg =
      (codeRes.json && (codeRes.json.message || codeRes.json.error)) ||
      `device code request failed (HTTP ${codeRes.status || "network"})`;
    throw new ApiError(sanitize(String(msg)), { subtype: "upstream", status: codeRes.status });
  }

  const payload = codeRes.json.data;
  const deviceCode = payload.device_code;
  const userCode = payload.user_code;
  const verificationUri =
    payload.verification_uri_complete || payload.verification_uri;
  const expiresIn = Number(payload.expires_in) || 900;
  const intervalMs = Math.max(1, Number(payload.interval) || 5) * 1000;

  if (!deviceCode || !userCode || !verificationUri) {
    throw new ApiError("device code response missing required fields", {
      subtype: "upstream",
    });
  }

  return {
    deviceCode,
    userCode,
    verificationUri,
    expiresIn,
    intervalMs,
    expiresAt: Date.now() + expiresIn * 1000,
  };
}

function emitUserFacingCodes(userCode, verificationUri, noOpen) {
  // Human-oriented stderr only (not machine contract). Agents use JSON data.
  process.stderr.write(`\n  Device code: ${userCode}\n`);
  process.stderr.write(`  Open: ${verificationUri}\n\n`);
  if (!noOpen) {
    const opened = openUrl(verificationUri);
    if (!opened.ok) {
      process.stderr.write(
        `Could not open a browser automatically (${opened.error || "unknown"}). Open the URL above manually.\n`
      );
    } else {
      process.stderr.write("Opened the authorization page in your browser.\n");
    }
  } else {
    process.stderr.write("Browser auto-open disabled (--no-open).\n");
  }
}

function successLoginPayload(userId, username) {
  return {
    success: true,
    logged_in: true,
    user_id: userId ? Number(userId) || userId : null,
    username: username || null,
    access_token: accessTokenMeta(),
    message: "Logged in. System access token stored securely.",
  };
}

async function runStatus(ctx) {
  const verify = hasFlag(ctx.args || [], ctx.flags || {}, "verify");
  const base = resolveBaseUrl(ctx.flags && ctx.flags.baseUrl);
  const user = resolveUserId();
  const token = resolveAccessToken();
  const present = !isBlank(token.value);

  /** @type {Record<string, unknown>} */
  const data = {
    base_url: base.value || null,
    base_url_source: base.source,
    user_id: user.value || null,
    user_id_source: user.source,
    access_token: {
      present,
      length: present ? String(token.value).length : 0,
      source: token.source || null,
    },
    configured: Boolean(base.value && present),
    verified: null,
  };

  if (verify) {
    if (!base.value || !present) {
      throw new AuthError("cannot verify: base_url or access token missing", {
        subtype: "token_missing",
        recovery: { command: "omnimux", args: ["login", "--no-wait"] },
      });
    }
    try {
      await apiRequest(
        { baseUrl: base.value, accessToken: token.value, userId: user.value || "" },
        "GET",
        "/api/user/self"
      );
      data.verified = true;
    } catch (err) {
      if (err && err.type === "auth") {
        data.verified = false;
        throw err;
      }
      throw err;
    }
  }

  ctx.out.ok(data);
}

async function run(ctx) {
  const args = ctx.args || [];
  const flags = ctx.flags || {};

  // login status [--verify]
  if (args[0] === "status") {
    return runStatus({ ...ctx, args: args.slice(1) });
  }

  const noOpen = hasFlag(args, flags, "no-open");
  const noWait = hasFlag(args, flags, "no-wait");
  const resumeId = flagValue(args, "resume") || (flags.resume ? String(flags.resume) : "");

  if (resumeId) {
    const flow = loadPendingFlow(resumeId);
    const result = await pollUntilDone(
      flow.base_url,
      flow.device_code,
      Number(flow.expires_at),
      Number(flow.interval_ms) || 5000
    );
    deletePendingFlow(resumeId);
    ctx.out.ok(successLoginPayload(result.userId, result.username));
    return;
  }

  const baseUrl = ctx.auth && ctx.auth.baseUrl;
  if (!baseUrl) {
    throw new UsageError("base_url is required for login", {
      recovery: { command: "omnimux", args: ["config", "set", "base_url", "<url>"] },
    });
  }

  // Agents should not see multi-line prose on stderr when using --no-wait;
  // keep a one-liner only for interactive default.
  if (!noWait) {
    process.stderr.write(
      "Starting device login. Successful login replaces your existing system access token.\n"
    );
  }

  const started = await startDeviceCode(baseUrl);

  if (noWait) {
    const flowId = savePendingFlow({
      deviceCode: started.deviceCode,
      userCode: started.userCode,
      verificationUri: started.verificationUri,
      expiresAt: started.expiresAt,
      intervalMs: started.intervalMs,
      baseUrl,
    });
    // Optional browser open for hybrid use; still no device_code on wire.
    if (!noOpen) {
      openUrl(started.verificationUri);
    }
    ctx.out.ok({
      verification_url: started.verificationUri,
      user_code: started.userCode,
      expires_in: started.expiresIn,
      interval: Math.round(started.intervalMs / 1000),
      flow_id: flowId,
      message: "Open verification_url; after user authorizes, run login --resume <flow_id>",
      next_step: `omnimux login --resume ${flowId}`,
    });
    return;
  }

  emitUserFacingCodes(started.userCode, started.verificationUri, noOpen);
  process.stderr.write("Waiting for authorization...\n");

  const result = await pollUntilDone(
    baseUrl,
    started.deviceCode,
    started.expiresAt,
    started.intervalMs
  );
  ctx.out.ok(successLoginPayload(result.userId, result.username));
}

module.exports = { run };
