const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const BIN = path.join(__dirname, "..", "src", "bin.js");
const RUNTIME = process.execPath;

function runCli(args, env = {}) {
  const configDir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-cli-voices-"));
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

test("voices default returns success envelope with total 509", () => {
  const r = runCli(["voices"]);
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stderr, "");
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.meta.command, "voices");
  assert.equal(env.data.total, 509);
  assert.equal(env.data.count, 509);
  assert.ok(Array.isArray(env.data.items));
  assert.equal(env.data.items.length, 509);
});

test("voices --query 顾姐 filters correctly", () => {
  const r = runCli(["voices", "--query", "顾姐"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.count, 2);
  const names = env.data.items.map((i) => i.name);
  assert.ok(names.every((n) => n === "顾姐"));
});

test("voices --query 顾姐 --model seed-audio-1.0 filters to 2.0 voice", () => {
  const r = runCli(["voices", "--query", "顾姐", "--model", "seed-audio-1.0"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.count, 1);
  const item = env.data.items[0];
  assert.equal(item.name, "顾姐");
  assert.equal(item.voice_type, "zh_female_gujie_uranus_bigtts");
  assert.equal(item.resource_id, "seed-tts-2.0");
  assert.equal(item.recommended_model, "seed-audio-1.0");
});

test("voices --scene 电商带货 returns relevant voices", () => {
  const r = runCli(["voices", "--scene", "电商带货", "--limit", "5"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.ok(env.data.count > 0 && env.data.count <= 5);
  assert.equal(env.data.items.length, env.data.count);
});

test("voices --limit limits the output length", () => {
  const r = runCli(["voices", "--limit", "3"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.total, 509);
  assert.equal(env.data.count, 3);
  assert.equal(env.data.items.length, 3);
});

test("schema voices returns valid leaf definition", () => {
  const r = runCli(["schema", "voices"]);
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.command.name, "voices");
  assert.equal(env.data.command.auth, "none");
  assert.equal(env.data.command.risk, "read");
});
