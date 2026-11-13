/**
 * Access-token storage for the omnimux CLI.
 *
 * Canonical unified storage across all platforms:
 * ~/.config/omnimux/secrets.json (dir 0700, file 0600)
 *
 * Resolution: OMNIMUX_ACCESS_TOKEN env > secret store (secrets.json).
 * Secret values are never printed by this module.
 */

const fs = require("fs");
const path = require("path");
const { configDir, isBlank, ENV_ACCESS_TOKEN } = require("./config");

const SERVICE = "omnimux";
const ACCOUNT = "access_token";

function isDarwin() {
  return process.platform === "darwin";
}

function secretsPath() {
  return path.join(configDir(), "secrets.json");
}

// --- 0600 file store (Canonical) ---

function fileGet() {
  const p = secretsPath();
  if (!fs.existsSync(p)) return "";
  try {
    const stat = fs.statSync(p);
    if ((stat.mode & 0o777) !== 0o600) {
      try {
        fs.chmodSync(p, 0o600);
      } catch {
        // ignore chmod failure
      }
    }
    const raw = JSON.parse(fs.readFileSync(p, "utf8"));
    if (!raw || typeof raw !== "object") return "";

    // 1. Check active slot if configured
    if (raw.active_slot && raw.slots && typeof raw.slots === "object") {
      const slot = raw.slots[raw.active_slot];
      if (slot && typeof slot.access_token === "string" && !isBlank(slot.access_token)) {
        return String(slot.access_token).trim();
      }
    }

    // 2. Check flat access_token
    if (!isBlank(raw.access_token)) {
      return String(raw.access_token).trim();
    }

    return "";
  } catch {
    return "";
  }
}

function fileSet(value) {
  const dir = configDir();
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  }

  let raw = {};
  const p = secretsPath();
  if (fs.existsSync(p)) {
    try {
      raw = JSON.parse(fs.readFileSync(p, "utf8")) || {};
    } catch {
      raw = {};
    }
  }

  const token = String(value).trim();
  const slots = raw.slots && typeof raw.slots === "object" ? { ...raw.slots } : {};
  slots["cli:default"] = {
    ...(slots["cli:default"] && typeof slots["cli:default"] === "object" ? slots["cli:default"] : {}),
    access_token: token,
    updated_at: Date.now(),
  };

  const payload = {
    version: typeof raw.version === "number" ? raw.version : 1,
    active_slot: "cli:default",
    access_token: token,
    slots,
  };

  const tmpPath = path.join(dir, `.secrets.tmp-${process.pid}-${Date.now()}`);
  fs.writeFileSync(tmpPath, `${JSON.stringify(payload, null, 2)}\n`, { mode: 0o600 });
  fs.chmodSync(tmpPath, 0o600);
  fs.renameSync(tmpPath, p);
  fs.chmodSync(p, 0o600);
}

function fileDelete() {
  const p = secretsPath();
  if (!fs.existsSync(p)) return false;
  try {
    const raw = JSON.parse(fs.readFileSync(p, "utf8"));
    if (!raw || typeof raw !== "object") {
      fs.unlinkSync(p);
      return true;
    }

    const slots = raw.slots && typeof raw.slots === "object" ? { ...raw.slots } : {};
    delete slots["cli:default"];
    if (raw.active_slot === "cli:default") {
      delete raw.active_slot;
    }

    const remainingSlotKeys = Object.keys(slots).filter((k) => slots[k] && !isBlank(slots[k].access_token));
    if (remainingSlotKeys.length === 0) {
      delete raw.access_token;
      delete raw.slots;
      if (Object.keys(raw).filter((k) => k !== "version").length === 0) {
        fs.unlinkSync(p);
        return true;
      }
    } else {
      const nextActive = remainingSlotKeys[0];
      raw.active_slot = nextActive;
      raw.access_token = slots[nextActive].access_token;
      raw.slots = slots;
    }

    const dir = configDir();
    const tmpPath = path.join(dir, `.secrets.tmp-${process.pid}-${Date.now()}`);
    fs.writeFileSync(tmpPath, `${JSON.stringify(raw, null, 2)}\n`, { mode: 0o600 });
    fs.chmodSync(tmpPath, 0o600);
    fs.renameSync(tmpPath, p);
    fs.chmodSync(p, 0o600);
    return true;
  } catch {
    return false;
  }
}

// --- Public API ---

function backend() {
  return "file";
}

function getStoredToken() {
  return fileGet();
}

function setStoredToken(value) {
  if (isBlank(value)) {
    throw new Error("Refusing to store an empty access token");
  }
  fileSet(value);
}

function deleteStoredToken() {
  return fileDelete();
}

/**
 * Resolve the access token: env var wins over the store (one-off override).
 * @returns {{ value: string, source: string }}
 */
function resolveAccessToken() {
  if (!isBlank(process.env[ENV_ACCESS_TOKEN])) {
    return { value: String(process.env[ENV_ACCESS_TOKEN]).trim(), source: "environment" };
  }
  const stored = getStoredToken();
  if (!isBlank(stored)) {
    return { value: String(stored).trim(), source: backend() };
  }
  return { value: "", source: "unset" };
}

/** Presence/length only — never the secret value. */
function accessTokenMeta() {
  const { value, source } = resolveAccessToken();
  return {
    set: !isBlank(value),
    length: value ? value.length : 0,
    source,
    backend: backend(),
  };
}

module.exports = {
  SERVICE,
  ACCOUNT,
  isDarwin,
  backend,
  secretsPath,
  getStoredToken,
  setStoredToken,
  deleteStoredToken,
  resolveAccessToken,
  accessTokenMeta,
};
