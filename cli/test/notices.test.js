const { test } = require("node:test");
const assert = require("node:assert/strict");
const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const {
  composeNotices,
  buildUpdateNotice,
  buildSkillsNotice,
} = require("../src/lib/notices");
const skill = require("../src/lib/skill");

const BIN = path.join(__dirname, "..", "src", "bin.js");
const RUNTIME = process.execPath;

test("buildUpdateNotice for updated action", () => {
  const n = buildUpdateNotice("0.2.10", {
    skipped: false,
    action: "updated",
    previous_version: "0.2.10",
    latest_version: "0.2.11",
  });
  assert.ok(n);
  assert.equal(n.latest, "0.2.11");
  assert.equal(n.recovery.command, "omnimux");
});

test("buildUpdateNotice null when already latest-ish", () => {
  const n = buildUpdateNotice("0.2.11", {
    skipped: false,
    action: "already_latest",
    current_version: "0.2.11",
  });
  assert.equal(n, null);
});

test("composeNotices respects suppress env", () => {
  const prevU = process.env.OMNIMUX_NO_UPDATE_NOTIFIER;
  const prevS = process.env.OMNIMUX_NO_SKILLS_NOTIFIER;
  process.env.OMNIMUX_NO_UPDATE_NOTIFIER = "1";
  process.env.OMNIMUX_NO_SKILLS_NOTIFIER = "1";
  try {
    const n = composeNotices({
      cliVersion: "0.2.11",
      autoUpdateResult: {
        skipped: false,
        action: "updated",
        previous_version: "0.2.10",
        latest_version: "0.2.11",
      },
    });
    assert.equal(n, undefined);
  } finally {
    if (prevU === undefined) delete process.env.OMNIMUX_NO_UPDATE_NOTIFIER;
    else process.env.OMNIMUX_NO_UPDATE_NOTIFIER = prevU;
    if (prevS === undefined) delete process.env.OMNIMUX_NO_SKILLS_NOTIFIER;
    else process.env.OMNIMUX_NO_SKILLS_NOTIFIER = prevS;
  }
});

test("readSkill returns omnimux SKILL.md content from package", () => {
  const r = skill.readSkill("omnimux");
  assert.equal(r.success, true);
  assert.equal(r.skill_id, "omnimux");
  assert.match(r.content, /SKILL: omnimux|Output contract/);
  assert.ok(r.path.endsWith(`${path.sep}omnimux${path.sep}SKILL.md`));
});

test("readSkill unknown id fails", () => {
  const r = skill.readSkill("not-a-real-skill-xyz");
  assert.equal(r.success, false);
  assert.equal(r.code, "unknown_skill");
});

test("spawn skill read omnimux", () => {
  const configDir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-notice-"));
  const r = spawnSync(RUNTIME, [BIN, "skill", "read", "omnimux"], {
    encoding: "utf8",
    env: {
      ...process.env,
      OMNIMUX_CONFIG_DIR: configDir,
      OMNIMUX_AUTO_UPDATE: "0",
      OMNIMUX_NO_UPDATE_NOTIFIER: "1",
      OMNIMUX_NO_SKILLS_NOTIFIER: "1",
    },
  });
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.data.skill_id, "omnimux");
  assert.ok(env.data.content.includes("omnimux"));
  assert.equal(env.meta.command, "skill.read");
});

test("spawn help carries meta without notices when suppressed", () => {
  const configDir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-notice-"));
  const r = spawnSync(RUNTIME, [BIN, "help"], {
    encoding: "utf8",
    env: {
      ...process.env,
      OMNIMUX_CONFIG_DIR: configDir,
      OMNIMUX_AUTO_UPDATE: "0",
      OMNIMUX_NO_UPDATE_NOTIFIER: "1",
      OMNIMUX_NO_SKILLS_NOTIFIER: "1",
    },
  });
  assert.equal(r.status, 0, r.stderr);
  const env = JSON.parse(r.stdout);
  assert.equal(env.ok, true);
  assert.equal(env.meta.notices, undefined);
});
