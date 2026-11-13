const { test, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");

const { placeholderFor, injectPlaceholder } = require("../src/lib/inject");

let dir;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-inject-"));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

test("placeholderFor follows the strict format", () => {
  assert.equal(placeholderFor(42), "__OMNIMUX_TOKEN_42__");
});

test("injectPlaceholder replaces every occurrence", () => {
  const file = path.join(dir, "config.json");
  fs.writeFileSync(
    file,
    '{"a":"__OMNIMUX_TOKEN_42__","b":"__OMNIMUX_TOKEN_42__"}'
  );
  injectPlaceholder(file, 42, "sk-real-key-value");
  const out = JSON.parse(fs.readFileSync(file, "utf8"));
  assert.equal(out.a, "sk-real-key-value");
  assert.equal(out.b, "sk-real-key-value");
});

test("injectPlaceholder preserves file mode", () => {
  const file = path.join(dir, "app.env");
  fs.writeFileSync(file, "KEY=__OMNIMUX_TOKEN_7__\n", { mode: 0o640 });
  injectPlaceholder(file, 7, "sk-real-key-value");
  const mode = fs.statSync(file).mode & 0o777;
  assert.equal(mode, 0o640);
  assert.equal(fs.readFileSync(file, "utf8"), "KEY=sk-real-key-value\n");
});

test("injectPlaceholder errors when placeholder is absent", () => {
  const file = path.join(dir, "config.json");
  fs.writeFileSync(file, '{"a":"nothing"}');
  assert.throws(() => injectPlaceholder(file, 42, "sk-x"), /Placeholder __OMNIMUX_TOKEN_42__ not found/);
  // Original content untouched
  assert.equal(fs.readFileSync(file, "utf8"), '{"a":"nothing"}');
});

test("injectPlaceholder errors on missing file and never leaks the secret", () => {
  const missing = path.join(dir, "nope.json");
  assert.throws(
    () => injectPlaceholder(missing, 1, "sk-super-secret-value"),
    (err) => {
      assert.match(err.message, /File not found/);
      assert.ok(!err.message.includes("sk-super-secret-value"));
      return true;
    }
  );
});
