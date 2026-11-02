const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const { enforceRiskGate } = require("../src/lib/risk");
const { UsageError, ConfirmationRequiredError } = require("../src/lib/errors");

const BIN = path.join(__dirname, "..", "src", "bin.js");
const RUNTIME = process.execPath;

function runCli(args, env = {}) {
  const configDir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-risk-"));
  return spawnSync(RUNTIME, [BIN, ...args], {
    encoding: "utf8",
    env: {
      ...process.env,
      OMNIMUX_CONFIG_DIR: configDir,
      OMNIMUX_AUTO_UPDATE: "0",
      OMNIMUX_ACCESS_TOKEN: "",
      OMNIMUX_USER_ID: "",
      ...env,
    },
  });
}

test("enforceRiskGate: dry-run unsupported fails closed", () => {
  assert.throws(
    () => enforceRiskGate("tokens.exec", { dryRun: true }),
    (err) => err instanceof UsageError && err.subtype === "dry_run_unsupported"
  );
});

test("enforceRiskGate: high-risk without yes throws confirmation", () => {
  assert.throws(
    () => enforceRiskGate("tokens.apply", {}),
    (err) => err instanceof ConfirmationRequiredError && err.exitCode === 10
  );
});

test("enforceRiskGate: high-risk with dry-run allowed", () => {
  const r = enforceRiskGate("tokens.apply", { dryRun: true });
  assert.equal(r.dryRun, true);
});

test("enforceRiskGate: update --check skips confirmation", () => {
  const r = enforceRiskGate("update", { check: true });
  assert.ok(r.leaf);
});

test("spawn: tokens create --dry-run returns preview without auth", () => {
  const r = runCli(["tokens", "create", "demo", "--dry-run", "--quota=1"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.dry_run, true);
  assert.equal(env.data.would, "POST /api/token/");
  assert.equal(env.data.body.name, "demo");
});

test("spawn: tokens apply without --yes is exit 10", () => {
  const r = runCli(["tokens", "apply", "1", "file.txt"]);
  assert.equal(r.status, 10);
  const env = JSON.parse(r.stderr);
  assert.equal(env.ok, false);
  assert.equal(env.error.type, "confirmation");
  assert.equal(env.error.subtype, "confirmation_required");
  assert.equal(env.error.risk, "high-risk-write");
});

test("spawn: tokens apply --dry-run previews without write", () => {
  const r = runCli(["tokens", "apply", "9", "cfg.env", "--dry-run"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.data.dry_run, true);
  assert.equal(env.data.token_id, 9);
  assert.match(env.data.placeholder, /OMNIMUX_TOKEN_9/);
});

test("spawn: tokens exec --dry-run is dry_run_unsupported", () => {
  const r = runCli(["tokens", "exec", "1", "--dry-run", "--", "echo", "x"]);
  assert.equal(r.status, 3);
  const env = JSON.parse(r.stderr);
  assert.equal(env.error.subtype, "dry_run_unsupported");
});
