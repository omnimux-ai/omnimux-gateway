const { test, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");

const secrets = require("../src/lib/secrets");

let dir;
const touchedEnv = ["OMNIMUX_CONFIG_DIR", "OMNIMUX_ACCESS_TOKEN"];
let savedEnv;

beforeEach(() => {
  savedEnv = {};
  for (const key of touchedEnv) {
    savedEnv[key] = process.env[key];
    delete process.env[key];
  }
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-secrets-"));
  process.env.OMNIMUX_CONFIG_DIR = dir;
});

afterEach(() => {
  for (const key of touchedEnv) {
    if (savedEnv[key] === undefined) delete process.env[key];
    else process.env[key] = savedEnv[key];
  }
  fs.rmSync(dir, { recursive: true, force: true });
});

test("backend is always 'file'", () => {
  assert.equal(secrets.backend(), "file");
});

test("setStoredToken creates 0600 secrets.json with slots and top-level token", () => {
  secrets.setStoredToken("test-pat-12345");
  const p = path.join(dir, "secrets.json");
  assert.equal(fs.existsSync(p), true);

  const stat = fs.statSync(p);
  assert.equal(stat.mode & 0o777, 0o600);

  const raw = JSON.parse(fs.readFileSync(p, "utf8"));
  assert.equal(raw.version, 1);
  assert.equal(raw.active_slot, "cli:default");
  assert.equal(raw.access_token, "test-pat-12345");
  assert.equal(raw.slots["cli:default"].access_token, "test-pat-12345");

  assert.equal(secrets.getStoredToken(), "test-pat-12345");
});

test("getStoredToken reads desktop slot if configured", () => {
  const p = path.join(dir, "secrets.json");
  const payload = {
    version: 1,
    active_slot: "desktop:default",
    access_token: "token-from-desktop",
    slots: {
      "desktop:default": {
        access_token: "token-from-desktop",
        updated_at: 12345678,
      },
    },
  };
  fs.writeFileSync(p, JSON.stringify(payload, null, 2), { mode: 0o600 });

  assert.equal(secrets.getStoredToken(), "token-from-desktop");
});

test("setStoredToken preserves existing desktop slot and updates cli:default", () => {
  const p = path.join(dir, "secrets.json");
  const initial = {
    version: 1,
    active_slot: "desktop:default",
    access_token: "token-desktop",
    slots: {
      "desktop:default": {
        access_token: "token-desktop",
        updated_at: 1000,
      },
    },
  };
  fs.writeFileSync(p, JSON.stringify(initial, null, 2), { mode: 0o600 });

  secrets.setStoredToken("token-cli");

  const raw = JSON.parse(fs.readFileSync(p, "utf8"));
  assert.equal(raw.active_slot, "cli:default");
  assert.equal(raw.access_token, "token-cli");
  assert.equal(raw.slots["desktop:default"].access_token, "token-desktop");
  assert.equal(raw.slots["cli:default"].access_token, "token-cli");
  assert.equal(secrets.getStoredToken(), "token-cli");
});

test("deleteStoredToken removes cli:default slot and falls back to desktop slot", () => {
  const p = path.join(dir, "secrets.json");
  const initial = {
    version: 1,
    active_slot: "cli:default",
    access_token: "token-cli",
    slots: {
      "desktop:default": {
        access_token: "token-desktop",
        updated_at: 1000,
      },
      "cli:default": {
        access_token: "token-cli",
        updated_at: 2000,
      },
    },
  };
  fs.writeFileSync(p, JSON.stringify(initial, null, 2), { mode: 0o600 });

  const deleted = secrets.deleteStoredToken();
  assert.equal(deleted, true);

  const raw = JSON.parse(fs.readFileSync(p, "utf8"));
  assert.equal(raw.active_slot, "desktop:default");
  assert.equal(raw.access_token, "token-desktop");
  assert.equal(raw.slots["cli:default"], undefined);
  assert.equal(raw.slots["desktop:default"].access_token, "token-desktop");
  assert.equal(secrets.getStoredToken(), "token-desktop");
});

test("deleteStoredToken unlinks file when no slots remain", () => {
  secrets.setStoredToken("token-cli-only");
  const p = path.join(dir, "secrets.json");
  assert.equal(fs.existsSync(p), true);

  const deleted = secrets.deleteStoredToken();
  assert.equal(deleted, true);
  assert.equal(fs.existsSync(p), false);
  assert.equal(secrets.getStoredToken(), "");
});

test("resolveAccessToken priority: env > secrets.json", () => {
  secrets.setStoredToken("token-from-file");

  process.env.OMNIMUX_ACCESS_TOKEN = "token-from-env";
  let resolved = secrets.resolveAccessToken();
  assert.equal(resolved.value, "token-from-env");
  assert.equal(resolved.source, "environment");

  delete process.env.OMNIMUX_ACCESS_TOKEN;
  resolved = secrets.resolveAccessToken();
  assert.equal(resolved.value, "token-from-file");
  assert.equal(resolved.source, "file");
});
