/**
 * Contract smoke tests against a live OmniMux/new-api instance.
 *
 * These run ONLY when OMNIMUX_TEST_BASE_URL is set; otherwise every test is
 * skipped so local runs without an instance stay green.
 *
 *   OMNIMUX_TEST_BASE_URL=http://localhost:7001 \
 *   OMNIMUX_ACCESS_TOKEN=... OMNIMUX_USER_ID=... \
 *   bun test test/contract.test.js
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");

const { spawnSync } = require("child_process");
const path = require("path");

const BASE = process.env.OMNIMUX_TEST_BASE_URL;
const BIN = path.join(__dirname, "..", "src", "bin.js");
const RUNTIME = process.env.OMNIMUX_TEST_RUNTIME || process.execPath;

const hasBase = !!BASE;
const hasAuth = hasBase && !!process.env.OMNIMUX_ACCESS_TOKEN && !!process.env.OMNIMUX_USER_ID;

// Use test.skip conditionally — bun test does not honor { skip: "reason" }.
const testWithBase = hasBase ? test : test.skip;
const testWithAuth = hasAuth ? test : test.skip;

function runCli(args) {
  const r = spawnSync(RUNTIME, [BIN, "--base-url", BASE, ...args], {
    encoding: "utf8",
    env: process.env,
  });
  return r;
}

testWithBase("pricing returns a JSON payload from the public API", () => {
  const r = runCli(["pricing"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(env.data && typeof env.data === "object");
});

testWithAuth("models returns a JSON payload with auth", () => {
  const r = runCli(["models"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(env.data && typeof env.data === "object");
});

testWithAuth("balance returns quota fields with auth", () => {
  const r = runCli(["balance"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok("derived" in env.data);
  assert.ok(!r.stdout.match(/sk-[A-Za-z0-9_\-]{8,}/), "stdout must never contain a full sk- key");
});
