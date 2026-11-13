"use strict";

const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");

const skillRoot = path.join(__dirname, "..", "skill");

const EXPECTED_BUSINESS = [
  "recreate-viral-video",
  "slideshow-creation",
  "video-script-creation",
  "video-analysis",
  "video-creation",
  "video-prompt-generation",
  "video-storyboard",
  "video-storyboard-planning",
  "ugc-commerce-video-skill",
  "image-creation",
  "image-recreation",
  "social-card",
  "product-image-set",
  "multi-angle-product-image",
  "a-plus-content",
  "infographic",
];

test("package skill tree is flat with full P0-P2 catalog", () => {
  assert.equal(fs.existsSync(path.join(skillRoot, "packs")), false);
  assert.ok(fs.existsSync(path.join(skillRoot, "manifest.json")));
  const manifest = JSON.parse(fs.readFileSync(path.join(skillRoot, "manifest.json"), "utf8"));
  assert.ok(Array.isArray(manifest.skills));
  // shared + 16 business
  assert.ok(manifest.skills.length >= 17, `expected >=17 skills, got ${manifest.skills.length}`);

  const ids = new Set(manifest.skills.map((s) => s.id));
  assert.ok(ids.has("omnimux"));
  for (const id of EXPECTED_BUSINESS) {
    assert.ok(ids.has(id), `manifest missing ${id}`);
  }

  for (const s of manifest.skills) {
    const dir = path.join(skillRoot, s.path);
    assert.ok(fs.existsSync(path.join(dir, "SKILL.md")), `${s.id} SKILL.md`);
    const body = fs.readFileSync(path.join(dir, "SKILL.md"), "utf8");
    assert.equal(body.split("\n")[0], "---", `${s.id} frontmatter`);
    assert.match(body, new RegExp(`^name:\\s*${s.id}\\s*$`, "m"));
  }
});

test("business skills are methodology-first with OMNIMUX.md", () => {
  for (const id of EXPECTED_BUSINESS) {
    const body = fs.readFileSync(path.join(skillRoot, id, "SKILL.md"), "utf8");
    assert.match(body, /codeExecutionCore:\s*false/, id);
    assert.match(body, /OmniMux shared|omnimux/, id);
    assert.equal(fs.existsSync(path.join(skillRoot, id, "scripts")), false, id);
    assert.ok(fs.existsSync(path.join(skillRoot, id, "OMNIMUX.md")), id);
  }
});

test("no Gxgen exclusive relatedTools blocks in business skills", () => {
  for (const id of EXPECTED_BUSINESS) {
    const body = fs.readFileSync(path.join(skillRoot, id, "SKILL.md"), "utf8");
    assert.doesNotMatch(body, /^  relatedTools:/m, id);
  }
});
