/**
 * omnimux config — view and set CLI configuration.
 *
 *   omnimux config                          secret-safe status
 *   omnimux config get <key>                base_url | user_id | access_token | auto_update
 *   omnimux config set <key> <value>        base_url / user_id / auto_update -> config.json
 *                                           access_token -> Keychain (macOS) or 0600 secrets.json
 *   omnimux config unset <key>
 *
 * Secrets never go into config.json and are never printed; `get access_token`
 * returns presence/length metadata only.
 */

const {
  configPath,
  loadConfig,
  saveConfig,
  resolveBaseUrl,
  resolveUserId,
  resolveAutoUpdate,
  safeHost,
  NON_SECRET_KEYS,
  BOOL_CONFIG_KEYS,
  coerceBool,
  ENV_BASE_URL,
  ENV_USER_ID,
  ENV_ACCESS_TOKEN,
  ENV_AUTO_UPDATE,
} = require("../lib/config");
const {
  setStoredToken,
  deleteStoredToken,
  accessTokenMeta,
} = require("../lib/secrets");
const { printJson, UsageError } = require("../lib/output");

const SETTABLE_KEYS = [...NON_SECRET_KEYS, "access_token"];

function status(ctx) {
  const base = resolveBaseUrl(ctx.flags.baseUrl);
  const user = resolveUserId();
  const auto = resolveAutoUpdate(ctx.flags);
  printJson(
    {
      config_file: configPath(),
      base_url: { set: !!base.value, source: base.source, host: safeHost(base.value) },
      user_id: { set: !!user.value, source: user.source, value: user.value || null },
      auto_update: { value: auto.value, source: auto.source },
      access_token: accessTokenMeta(),
      env: {
        base_url: ENV_BASE_URL,
        user_id: ENV_USER_ID,
        access_token: ENV_ACCESS_TOKEN,
        auto_update: ENV_AUTO_UPDATE,
      },
    },
    ctx.flags
  );
}

function get(ctx) {
  const key = ctx.args[0];
  if (!SETTABLE_KEYS.includes(key)) {
    throw new UsageError(`unknown config key: ${key || "(missing)"} (expected: ${SETTABLE_KEYS.join(", ")})`);
  }
  if (key === "base_url") {
    const { value, source } = resolveBaseUrl(ctx.flags.baseUrl);
    printJson({ key, value: value || null, source }, ctx.flags);
  } else if (key === "user_id") {
    const { value, source } = resolveUserId();
    printJson({ key, value: value || null, source }, ctx.flags);
  } else if (key === "auto_update") {
    const { value, source } = resolveAutoUpdate(ctx.flags);
    printJson({ key, value, source }, ctx.flags);
  } else {
    // access_token: metadata only, never the value.
    printJson({ key, ...accessTokenMeta() }, ctx.flags);
  }
}

function set(ctx) {
  const [key, ...rest] = ctx.args;
  const value = rest.join(" ");
  if (!SETTABLE_KEYS.includes(key) || !value) {
    throw new UsageError(`usage: omnimux config set <${SETTABLE_KEYS.join("|")}> <value>`);
  }
  if (key === "access_token") {
    setStoredToken(value);
    printJson({ success: true, key, ...accessTokenMeta() }, ctx.flags);
    return;
  }
  if (BOOL_CONFIG_KEYS.includes(key)) {
    const b = coerceBool(value);
    if (b === null) {
      throw new UsageError(
        `invalid boolean for ${key}: ${value} (expected: true|false|1|0|on|off|yes|no)`
      );
    }
    const cfg = loadConfig();
    cfg[key] = b;
    saveConfig(cfg);
    printJson({ success: true, key, value: b }, ctx.flags);
    return;
  }
  const cfg = loadConfig();
  cfg[key] = key === "base_url" ? value.replace(/\/+$/, "") : value;
  saveConfig(cfg);
  printJson({ success: true, key, value: cfg[key] }, ctx.flags);
}

function unset(ctx) {
  const key = ctx.args[0];
  if (!SETTABLE_KEYS.includes(key)) {
    throw new UsageError(`unknown config key: ${key || "(missing)"} (expected: ${SETTABLE_KEYS.join(", ")})`);
  }
  if (key === "access_token") {
    deleteStoredToken();
    printJson({ success: true, key, ...accessTokenMeta() }, ctx.flags);
    return;
  }
  const cfg = loadConfig();
  delete cfg[key];
  saveConfig(cfg);
  printJson({ success: true, key, removed: true }, ctx.flags);
}

async function run(ctx) {
  const sub = ctx.args[0];
  const rest = { ...ctx, args: ctx.args.slice(1) };
  switch (sub) {
    case undefined:
    case "status":
      status(ctx);
      break;
    case "get":
      get(rest);
      break;
    case "set":
      set(rest);
      break;
    case "unset":
      unset(rest);
      break;
    default:
      throw new UsageError(`unknown config subcommand: ${sub} (expected: status, get, set, unset)`);
  }
}

module.exports = { run };
