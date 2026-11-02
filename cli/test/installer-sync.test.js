"use strict";

/**
 * Installer-copy parity guard.
 *
 * `https://omnimux.ai/install.sh` and `/install.ps1` are served from the
 * embedded `web/public/` copies, while `cli/scripts/` is the source of truth.
 * Keep both pairs byte-identical so a CLI installer change cannot ship to
 * GitHub Releases/npm while the public one-liner stays stale (or vice versa).
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");

const repoRoot = path.join(__dirname, "..", "..");

for (const name of ["install.sh", "install.ps1"]) {
  test(`web/public/${name} mirrors cli/scripts/${name}`, () => {
    const source = fs.readFileSync(path.join(repoRoot, "cli", "scripts", name));
    const served = fs.readFileSync(path.join(repoRoot, "web", "public", name));
    assert.ok(
      source.equals(served),
      `web/public/${name} is stale; run: cp cli/scripts/${name} web/public/${name}`
    );
  });
}
