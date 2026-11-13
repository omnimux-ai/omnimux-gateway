const { test, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");

const config = require("../src/lib/config");

let dir;
const touchedEnv = [
  "OMNIMUX_CONFIG_DIR",
  "OMNIMUX_BASE_URL",
  "OMNIMUX_USER_ID",
  "OMNIMUX_AUTO_UPDATE",
];
let savedEnv;

beforeEach(() => {
  savedEnv = {};
  for (const key of touchedEnv) {
    savedEnv[key] = process.env[key];
    delete process.env[key];
  }
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-config-"));
  process.env.OMNIMUX_CONFIG_DIR = dir;
});

afterEach(() => {
  for (const key of touchedEnv) {
    if (savedEnv[key] === undefined) delete process.env[key];
    else process.env[key] = savedEnv[key];
  }
  fs.rmSync(dir, { recursive: true, force: true });
});

test("default base URL constant points at the official instance", () => {
  assert.equal(config.OMNIMUX_DEFAULT_BASE_URL, "https://omnimux.ai");
});

test("resolveBaseUrl priority: flag > env > config file", () => {
  config.saveConfig({ base_url: "https://from-config.example.com" });
  process.env.OMNIMUX_BASE_URL = "https://from-env.example.com";

  const fromFlag = config.resolveBaseUrl("https://from-flag.example.com/");
  assert.equal(fromFlag.value, "https://from-flag.example.com");
  assert.equal(fromFlag.source, "flag");

  const fromEnv = config.resolveBaseUrl();
  assert.equal(fromEnv.value, "https://from-env.example.com");
  assert.equal(fromEnv.source, "environment");

  delete process.env.OMNIMUX_BASE_URL;
  const fromConfig = config.resolveBaseUrl();
  assert.equal(fromConfig.value, "https://from-config.example.com");
  assert.equal(fromConfig.source, "config");
});

test("requireBaseUrl falls back to the built-in default when unset", () => {
  const { value, source } = config.requireBaseUrl();
  assert.equal(value, "https://omnimux.ai");
  assert.equal(source, "default");
});

test("resolveBaseUrl returns the built-in default when nothing is configured", () => {
  const { value, source } = config.resolveBaseUrl();
  assert.equal(value, "https://omnimux.ai");
  assert.equal(source, "default");
});

test("resolveUserId priority: env > config file", () => {
  config.saveConfig({ user_id: "7" });
  assert.deepEqual(config.resolveUserId(), { value: "7", source: "config" });
  process.env.OMNIMUX_USER_ID = "9";
  assert.deepEqual(config.resolveUserId(), { value: "9", source: "environment" });
});

test("saveConfig never persists secret-looking keys and writes mode 0600", () => {
  config.saveConfig({
    base_url: "https://example.com",
    user_id: "3",
    access_token: "should-not-be-stored",
  });
  const file = config.configPath();
  const raw = JSON.parse(fs.readFileSync(file, "utf8"));
  assert.deepEqual(raw, { base_url: "https://example.com", user_id: "3" });
  const mode = fs.statSync(file).mode & 0o777;
  assert.equal(mode, 0o600);
});

test("loadConfig ignores unknown and blank keys", () => {
  fs.writeFileSync(
    config.configPath(),
    JSON.stringify({ base_url: " https://a.example.com ", junk: 1, user_id: "  " })
  );
  assert.deepEqual(config.loadConfig(), { base_url: "https://a.example.com" });
});

test("loadConfig tolerates corrupt config files", () => {
  fs.writeFileSync(config.configPath(), "not json {");
  assert.deepEqual(config.loadConfig(), {});
});

test("safeHost extracts host and tolerates invalid URLs", () => {
  assert.equal(config.safeHost("https://api.example.com/v1"), "api.example.com");
  assert.equal(config.safeHost("not a url"), "(invalid_url)");
  assert.equal(config.safeHost(""), "");
});

test("resolveAutoUpdate defaults to true when unset", () => {
  const { value, source } = config.resolveAutoUpdate();
  assert.equal(value, true);
  assert.equal(source, "default");
});

test("resolveAutoUpdate priority: flag > env > config", () => {
  config.saveConfig({ auto_update: true });
  process.env.OMNIMUX_AUTO_UPDATE = "0";

  const fromFlag = config.resolveAutoUpdate({ noAutoUpdate: true });
  assert.deepEqual(fromFlag, { value: false, source: "flag" });

  const fromEnv = config.resolveAutoUpdate();
  assert.deepEqual(fromEnv, { value: false, source: "environment" });

  delete process.env.OMNIMUX_AUTO_UPDATE;
  const fromConfig = config.resolveAutoUpdate();
  assert.deepEqual(fromConfig, { value: true, source: "config" });

  config.saveConfig({ auto_update: false });
  assert.deepEqual(config.resolveAutoUpdate(), { value: false, source: "config" });
});

test("saveConfig persists auto_update false (boolean keys)", () => {
  config.saveConfig({ base_url: "https://example.com", auto_update: false });
  const raw = JSON.parse(fs.readFileSync(config.configPath(), "utf8"));
  assert.equal(raw.auto_update, false);
  assert.equal(config.loadConfig().auto_update, false);
});

test("coerceBool accepts common truthy/falsey spellings", () => {
  assert.equal(config.coerceBool("on"), true);
  assert.equal(config.coerceBool("OFF"), false);
  assert.equal(config.coerceBool("yes"), true);
  assert.equal(config.coerceBool("no"), false);
  assert.equal(config.coerceBool("maybe"), null);
});
