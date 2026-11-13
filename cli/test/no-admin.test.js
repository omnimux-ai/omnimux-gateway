/**
 * Admin-isolation guard.
 *
 * The omnimux CLI is a user-level product surface: no admin/root endpoint may
 * ever appear in the shipped source. This test scans cli/src/ and asserts
 * that none of the admin route prefixes guarded by AdminAuth()/RootAuth() in
 * router/api-router.go (and router/channel-router.go / authz-router.go) show
 * up as string literals.
 *
 * Keep the blacklist in sync with the router files when upstream adds admin
 * route groups.
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");

// Prefixes of route groups guarded by AdminAuth() / RootAuth() in
// router/api-router.go, router/channel-router.go, router/authz-router.go.
const ADMIN_PREFIX_BLACKLIST = [
  "/api/channel",
  "/api/authz",
  "/api/redemption",
  "/api/group",
  "/api/prefill_group",
  "/api/vendors",
  "/api/models",
  "/api/deployments",
  "/api/subscription/admin",
  "/api/option",
  "/api/custom-oauth-provider",
  "/api/performance",
  "/api/ratio_sync",
  "/api/system-task",
  "/api/system-info",
  "/api/hosted/admin",
];

function collectFiles(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...collectFiles(full));
    else if (entry.isFile() && entry.name.endsWith(".js")) out.push(full);
  }
  return out;
}

const SRC_DIR = path.join(__dirname, "..", "src");

test("cli/src contains no admin endpoint prefixes", () => {
  const files = collectFiles(SRC_DIR);
  assert.ok(files.length > 0, "expected to scan at least one source file");
  const violations = [];
  for (const file of files) {
    const content = fs.readFileSync(file, "utf8");
    for (const prefix of ADMIN_PREFIX_BLACKLIST) {
      if (content.includes(prefix)) {
        violations.push(`${path.relative(SRC_DIR, file)} contains ${prefix}`);
      }
    }
  }
  assert.deepEqual(violations, []);
});
