/**
 * Spawned-process contract for Agent RPC envelopes.
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const BIN = path.join(__dirname, "..", "src", "bin.js");
const RUNTIME = process.execPath;

function runCli(args, env = {}) {
  const configDir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-cli-env-"));
  const r = spawnSync(RUNTIME, [BIN, ...args], {
    encoding: "utf8",
    env: {
      ...process.env,
      OMNIMUX_CONFIG_DIR: configDir,
      OMNIMUX_AUTO_UPDATE: "0",
      OMNIMUX_ACCESS_TOKEN: "",
      OMNIMUX_USER_ID: "",
      OMNIMUX_BASE_URL: "",
      ...env,
    },
  });
  return r;
}

test("version returns success envelope", () => {
  const r = runCli(["--version"]);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stderr, "");
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(env.data.version);
  assert.equal(env.meta.command, "version");
  assert.equal(env.meta.contract_version, 1);
});

test("unknown command returns usage error envelope on stderr", () => {
  const r = runCli(["not-a-real-command"]);
  assert.equal(r.status, 3);
  assert.equal(r.stdout, "");
  const env = JSON.parse(r.stderr);
  assert.equal(env.ok, false);
  assert.equal(env.error.type, "usage");
  assert.equal(env.error.subtype, "unknown_command");
});

test("schema lists commands under data", () => {
  const r = runCli(["schema"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(Array.isArray(env.data.commands));
  assert.ok(env.data.commands.some((c) => c.name === "login"));
  assert.ok(env.data.commands.some((c) => c.name === "schema"));
});

test("schema tokens.create returns leaf detail", () => {
  const r = runCli(["schema", "tokens.create"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.data.command.name, "tokens.create");
  assert.equal(env.data.command.risk, "write");
});

test("full-auth command missing user_id returns config envelope exit 2", () => {
  // Force env token so host keychain cannot satisfy auth; leave user_id unset.
  const r = runCli(["models"], {
    OMNIMUX_BASE_URL: "https://example.invalid",
    OMNIMUX_ACCESS_TOKEN: "sk-test-envelope-only",
    OMNIMUX_USER_ID: "",
  });
  assert.equal(r.status, 2);
  const env = JSON.parse(r.stderr);
  assert.equal(env.ok, false);
  assert.equal(env.error.type, "config");
  assert.equal(env.error.subtype, "not_configured");
  assert.ok(env.error.recovery);
  assert.equal(env.error.recovery.command, "omnimux");
});

test("help returns JSON envelope with usage text", () => {
  const r = runCli(["help"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(typeof env.data.usage === "string");
  assert.ok(Array.isArray(env.data.commands));
});

test("login status returns machine-readable fields without secrets", () => {
  const r = runCli(["login", "status"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.meta.command, "login.status");
  assert.ok(env.data.access_token && typeof env.data.access_token.present === "boolean");
  assert.ok(!JSON.stringify(env).includes("sk-test"));
  // Never emit raw token value
  assert.equal(env.data.access_token.value, undefined);
});
