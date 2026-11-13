/**
 * Pending device-login flow store.
 *
 * Server `device_code` is never printed. Agents resume with opaque flow_id:
 *   omnimux login --no-wait  → data.flow_id
 *   omnimux login --resume <flow_id>
 */

const fs = require("fs");
const path = require("path");
const crypto = require("crypto");
const { configDir } = require("./config");
const { AuthError } = require("./errors");

function flowsDir() {
  return path.join(configDir(), "login-flows");
}

function flowPath(flowId) {
  // flow ids are hex; reject path traversal
  if (!/^[a-f0-9]{16,64}$/i.test(String(flowId || ""))) {
    throw new AuthError("invalid flow id", {
      subtype: "flow_not_found",
      hint: "start a new login with --no-wait",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  return path.join(flowsDir(), `${String(flowId).toLowerCase()}.json`);
}

/**
 * @param {{
 *   deviceCode: string,
 *   userCode: string,
 *   verificationUri: string,
 *   expiresAt: number,
 *   intervalMs: number,
 *   baseUrl: string,
 * }} record
 * @returns {string} flow_id
 */
function savePendingFlow(record) {
  const dir = flowsDir();
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  }
  const flowId = crypto.randomBytes(16).toString("hex");
  const body = {
    flow_id: flowId,
    device_code: record.deviceCode,
    user_code: record.userCode,
    verification_uri: record.verificationUri,
    expires_at: record.expiresAt,
    interval_ms: record.intervalMs,
    base_url: record.baseUrl,
    created_at: Date.now(),
  };
  fs.writeFileSync(flowPath(flowId), JSON.stringify(body) + "\n", { mode: 0o600 });
  return flowId;
}

/**
 * @param {string} flowId
 */
function loadPendingFlow(flowId) {
  const p = flowPath(flowId);
  if (!fs.existsSync(p)) {
    throw new AuthError("login flow not found or already completed", {
      subtype: "flow_not_found",
      hint: "start a new login with --no-wait",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  let raw;
  try {
    raw = JSON.parse(fs.readFileSync(p, "utf8"));
  } catch {
    throw new AuthError("login flow record is corrupt", {
      subtype: "flow_not_found",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  if (!raw || !raw.device_code || !raw.base_url) {
    throw new AuthError("login flow record incomplete", {
      subtype: "flow_not_found",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  if (raw.expires_at && Date.now() > Number(raw.expires_at)) {
    try {
      fs.unlinkSync(p);
    } catch {
      /* ignore */
    }
    throw new AuthError("login flow expired", {
      subtype: "login_expired",
      hint: "start a new login with --no-wait",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  return raw;
}

/**
 * @param {string} flowId
 */
function deletePendingFlow(flowId) {
  try {
    const p = flowPath(flowId);
    if (fs.existsSync(p)) fs.unlinkSync(p);
  } catch {
    /* ignore */
  }
}

module.exports = {
  savePendingFlow,
  loadPendingFlow,
  deletePendingFlow,
  flowsDir,
};
