/**
 * Non-secret configuration for the omnimux CLI.
 *
 * Simplified port of .agents/skills/newapi/scripts/env.js + profiles.js:
 * no multi-profile registry, no CI branches, no .env file chain.
 *
 * Resolution priority (non-secrets):
 *   base_url: --base-url flag > OMNIMUX_BASE_URL env > config.json > OMNIMUX_DEFAULT_BASE_URL
 *   user_id:  OMNIMUX_USER_ID env > config.json
 *   auto_update: --no-auto-update flag > OMNIMUX_AUTO_UPDATE env > config.json > default true
 *
 * Secrets (access token) never live here — see lib/secrets.js.
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { ConfigError } = require("./errors");

/**
 * Official OmniMux instance URL. Used when no --base-url flag,
 * OMNIMUX_BASE_URL env, or config.json base_url is set. Self-hosted users
 * override it with any of those.
 */
const OMNIMUX_DEFAULT_BASE_URL = "https://omnimux.ai";

/** Default: auto-update is on when unset. */
const OMNIMUX_DEFAULT_AUTO_UPDATE = true;

const ENV_BASE_URL = "OMNIMUX_BASE_URL";
const ENV_USER_ID = "OMNIMUX_USER_ID";
const ENV_ACCESS_TOKEN = "OMNIMUX_ACCESS_TOKEN";
const ENV_AUTO_UPDATE = "OMNIMUX_AUTO_UPDATE";

/** String non-secret keys stored in config.json */
const STRING_CONFIG_KEYS = ["base_url", "user_id"];
/** Boolean non-secret keys stored in config.json (false is a real value). */
const BOOL_CONFIG_KEYS = ["auto_update"];
/** All non-secret config keys (for set/unset/status). */
const NON_SECRET_KEYS = [...STRING_CONFIG_KEYS, ...BOOL_CONFIG_KEYS];

function isBlank(value) {
  return value === undefined || value === null || String(value).trim() === "";
}

/**
 * Parse a boolean config/env value.
 * @returns {boolean | null} null if unrecognized / blank
 */
function coerceBool(value) {
  if (value === undefined || value === null) return null;
  if (typeof value === "boolean") return value;
  const s = String(value).trim().toLowerCase();
  if (s === "") return null;
  if (["1", "true", "yes", "on", "enable", "enabled"].includes(s)) return true;
  if (["0", "false", "no", "off", "disable", "disabled"].includes(s)) return false;
  return null;
}

function configDir() {
  // Test/isolation override; not part of the user-facing configuration model.
  if (!isBlank(process.env.OMNIMUX_CONFIG_DIR)) {
    return String(process.env.OMNIMUX_CONFIG_DIR);
  }
  return path.join(os.homedir(), ".config", "omnimux");
}

function configPath() {
  return path.join(configDir(), "config.json");
}

function loadConfig() {
  const p = configPath();
  if (!fs.existsSync(p)) return {};
  try {
    const raw = JSON.parse(fs.readFileSync(p, "utf8"));
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
    const out = {};
    for (const key of STRING_CONFIG_KEYS) {
      if (!isBlank(raw[key])) out[key] = String(raw[key]).trim();
    }
    for (const key of BOOL_CONFIG_KEYS) {
      if (Object.prototype.hasOwnProperty.call(raw, key)) {
        const b = coerceBool(raw[key]);
        if (b !== null) out[key] = b;
      }
    }
    return out;
  } catch {
    return {};
  }
}

function saveConfig(cfg) {
  const dir = configDir();
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  }
  const clean = {};
  for (const key of STRING_CONFIG_KEYS) {
    if (!isBlank(cfg[key])) {
      clean[key] = String(cfg[key]).trim();
    }
  }
  for (const key of BOOL_CONFIG_KEYS) {
    if (typeof cfg[key] === "boolean") {
      clean[key] = cfg[key];
    }
  }
  // Never persist secrets even if a caller passes them.
  fs.writeFileSync(configPath(), JSON.stringify(clean, null, 2) + "\n", {
    mode: 0o600,
  });
  return configPath();
}

function normalizeBaseUrl(url) {
  return String(url || "").trim().replace(/\/+$/, "");
}

/**
 * Resolve the instance base URL.
 * @param {string} [flagValue] value of --base-url, if given
 * @returns {{ value: string, source: string }}
 */
function resolveBaseUrl(flagValue) {
  if (!isBlank(flagValue)) {
    return { value: normalizeBaseUrl(flagValue), source: "flag" };
  }
  if (!isBlank(process.env[ENV_BASE_URL])) {
    return { value: normalizeBaseUrl(process.env[ENV_BASE_URL]), source: "environment" };
  }
  const cfg = loadConfig();
  if (!isBlank(cfg.base_url)) {
    return { value: normalizeBaseUrl(cfg.base_url), source: "config" };
  }
  if (!isBlank(OMNIMUX_DEFAULT_BASE_URL)) {
    return { value: normalizeBaseUrl(OMNIMUX_DEFAULT_BASE_URL), source: "default" };
  }
  return { value: "", source: "unset" };
}

/**
 * Like resolveBaseUrl but throws a ConfigError with setup instructions
 * when no base URL is configured anywhere.
 */
function requireBaseUrl(flagValue) {
  const { value, source } = resolveBaseUrl(flagValue);
  if (value) return { value, source };
  throw new ConfigError(
    "base_url is not configured",
    {
      subtype: "not_configured",
      hint: "set base_url then retry",
      recovery: { command: "omnimux", args: ["config", "set", "base_url", "<url>"] },
    }
  );
}

/**
 * Resolve the user id for the New-Api-User header.
 * @returns {{ value: string, source: string }}
 */
function resolveUserId() {
  if (!isBlank(process.env[ENV_USER_ID])) {
    return { value: String(process.env[ENV_USER_ID]).trim(), source: "environment" };
  }
  const cfg = loadConfig();
  if (!isBlank(cfg.user_id)) {
    return { value: String(cfg.user_id).trim(), source: "config" };
  }
  return { value: "", source: "unset" };
}

function requireUserId() {
  const { value, source } = resolveUserId();
  if (value) return { value, source };
  throw new ConfigError(
    "user_id is not configured",
    {
      subtype: "not_configured",
      hint: "set user_id then retry",
      recovery: { command: "omnimux", args: ["config", "set", "user_id", "<id>"] },
    }
  );
}

function safeHost(baseUrl) {
  if (isBlank(baseUrl)) return "";
  try {
    return new URL(baseUrl).host || baseUrl;
  } catch {
    return "(invalid_url)";
  }
}

/**
 * Resolve whether background self-update is enabled.
 * Priority: --no-auto-update flag > OMNIMUX_AUTO_UPDATE env > config.json > default true.
 *
 * @param {{ noAutoUpdate?: boolean }} [flags]
 * @returns {{ value: boolean, source: string }}
 */
function resolveAutoUpdate(flags = {}) {
  if (flags && flags.noAutoUpdate) {
    return { value: false, source: "flag" };
  }
  if (!isBlank(process.env[ENV_AUTO_UPDATE])) {
    const b = coerceBool(process.env[ENV_AUTO_UPDATE]);
    if (b !== null) return { value: b, source: "environment" };
  }
  const cfg = loadConfig();
  if (typeof cfg.auto_update === "boolean") {
    return { value: cfg.auto_update, source: "config" };
  }
  return { value: OMNIMUX_DEFAULT_AUTO_UPDATE, source: "default" };
}

module.exports = {
  OMNIMUX_DEFAULT_BASE_URL,
  OMNIMUX_DEFAULT_AUTO_UPDATE,
  ENV_BASE_URL,
  ENV_USER_ID,
  ENV_ACCESS_TOKEN,
  ENV_AUTO_UPDATE,
  STRING_CONFIG_KEYS,
  BOOL_CONFIG_KEYS,
  NON_SECRET_KEYS,
  ConfigError,
  isBlank,
  coerceBool,
  configDir,
  configPath,
  loadConfig,
  saveConfig,
  normalizeBaseUrl,
  resolveBaseUrl,
  requireBaseUrl,
  resolveUserId,
  requireUserId,
  resolveAutoUpdate,
  safeHost,
};
