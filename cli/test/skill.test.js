"use strict";

const { test, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");

const skill = require("../src/lib/skill");

let dir;
let home;
let savedEnv;

function writeMiniTree(root) {
  fs.mkdirSync(path.join(root, "omnimux"), { recursive: true });
  fs.mkdirSync(path.join(root, "recreate-viral-video"), { recursive: true });
  fs.writeFileSync(
    path.join(root, "manifest.json"),
    JSON.stringify({
      version: 1,
      skills: [
        { id: "omnimux", path: "omnimux", role: "shared", required: true },
        {
          id: "recreate-viral-video",
          path: "recreate-viral-video",
          role: "business",
          requires: ["omnimux"],
        },
      ],
    })
  );
  fs.writeFileSync(path.join(root, "omnimux", "SKILL.md"), "---\nname: omnimux\n---\n# hub\n");
  fs.writeFileSync(
    path.join(root, "recreate-viral-video", "SKILL.md"),
    "---\nname: recreate-viral-video\n---\n# viral\n"
  );
}

beforeEach(() => {
  savedEnv = {
    OMNIMUX_CONFIG_DIR: process.env.OMNIMUX_CONFIG_DIR,
    OMNIMUX_SKILL_SOURCE: process.env.OMNIMUX_SKILL_SOURCE,
    OMNIMUX_HOME: process.env.OMNIMUX_HOME,
    HOME: process.env.HOME,
  };
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-skill-"));
  home = path.join(dir, "home");
  fs.mkdirSync(home, { recursive: true });
  process.env.OMNIMUX_CONFIG_DIR = path.join(dir, "cfg");
  process.env.OMNIMUX_HOME = home;
  process.env.HOME = home;
  const src = path.join(dir, "pkg-skill");
  writeMiniTree(src);
  process.env.OMNIMUX_SKILL_SOURCE = src;
});

afterEach(() => {
  for (const [k, v] of Object.entries(savedEnv)) {
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
  fs.rmSync(dir, { recursive: true, force: true });
});

test("install --user creates flat sibling skill dirs", () => {
  const result = skill.installSkill({ scope: "user", mode: "link" });
  assert.equal(result.success, true);
  assert.equal(result.installed.length, 2);
  const hub = skill.defaultTargetPath("user", "omnimux");
  const viral = skill.defaultTargetPath("user", "recreate-viral-video");
  assert.ok(fs.lstatSync(hub).isSymbolicLink());
  assert.ok(fs.lstatSync(viral).isSymbolicLink());
  assert.ok(fs.existsSync(path.join(hub, "SKILL.md")));
  assert.ok(fs.existsSync(path.join(viral, "SKILL.md")));
  const reg = skill.loadSkillTargets();
  assert.equal(reg.schema, 2);
  assert.equal(reg.targets.length, 2);
});

test("install --project copy writes per-skill meta", () => {
  const project = path.join(dir, "proj");
  fs.mkdirSync(project, { recursive: true });
  const result = skill.installSkill({
    scope: "project",
    mode: "copy",
    cwd: project,
    cliVersion: "0.2.7",
  });
  assert.equal(result.success, true);
  const viral = skill.defaultTargetPath("project", "recreate-viral-video", project);
  const meta = JSON.parse(fs.readFileSync(path.join(viral, skill.META_FILENAME), "utf8"));
  assert.equal(meta.cli_version, "0.2.7");
  assert.equal(meta.skill_id, "recreate-viral-video");
});

test("legacy whole-tree omnimux install is migrated on reinstall", () => {
  const project = path.join(dir, "legacy-proj");
  const skillsRoot = path.join(project, ".agents", "skills");
  fs.mkdirSync(skillsRoot, { recursive: true });
  // Simulate 0.2.6: single link to package skill root (contains multiple skills)
  const bundle = path.join(skillsRoot, "omnimux");
  fs.symlinkSync(process.env.OMNIMUX_SKILL_SOURCE, bundle, "dir");
  assert.equal(skill.isLegacyBundleInstall(bundle), true);

  const result = skill.installSkill({ scope: "project", mode: "link", cwd: project });
  assert.equal(result.success, true);
  assert.equal(result.legacy_bundle_migrated, true);
  assert.ok(fs.existsSync(skill.defaultTargetPath("project", "omnimux", project)));
  assert.ok(fs.existsSync(skill.defaultTargetPath("project", "recreate-viral-video", project)));
  // Flat hub must NOT be a bundle (no nested peer skill ids inside)
  const hubResolved = fs.realpathSync(skill.defaultTargetPath("project", "omnimux", project));
  assert.ok(fs.existsSync(path.join(hubResolved, "SKILL.md")));
  assert.equal(fs.existsSync(path.join(hubResolved, "recreate-viral-video")), false);
});

test("status reports catalog and agent_skills", () => {
  skill.installSkill({ scope: "user", mode: "link" });
  const st = skill.skillStatus();
  assert.equal(st.success, true);
  assert.ok(st.catalog.some((c) => c.id === "recreate-viral-video"));
  assert.ok(st.agent_skills.some((a) => a.skill_id === "recreate-viral-video"));
});

test("uninstall removes flat skills for scope", () => {
  skill.installSkill({ scope: "user", mode: "link" });
  const u = skill.uninstallSkill("user");
  assert.equal(u.success, true);
  assert.equal(fs.existsSync(skill.defaultTargetPath("user", "omnimux")), false);
  assert.equal(fs.existsSync(skill.defaultTargetPath("user", "recreate-viral-video")), false);
  assert.equal(skill.loadSkillTargets().targets.length, 0);
});
