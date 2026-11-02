/**
 * Global argv parsing: known globals stay on flags; command-local flags
 * pass through into args for subcommand parsers (social, tokens extras, …).
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");

const { parseArgs } = require("../src/bin.js");

test("passthrough: social connect --platform tiktok", () => {
  const { flags, args } = parseArgs([
    "social",
    "connect",
    "--platform",
    "tiktok",
    "--no-open",
  ]);
  assert.equal(flags.noOpen, true);
  assert.deepEqual(args, ["social", "connect", "--platform", "tiktok"]);
});

test("passthrough: --platform=tiktok equals form", () => {
  const { args } = parseArgs(["social", "connect", "--platform=tiktok"]);
  assert.deepEqual(args, ["social", "connect", "--platform=tiktok"]);
});

test("passthrough: social post multi flags", () => {
  const { flags, args } = parseArgs([
    "--pretty",
    "social",
    "post",
    "--accounts",
    "1,2",
    "--text",
    "hello",
    "--idempotency-key",
    "k1",
  ]);
  assert.equal(flags.pretty, true);
  assert.deepEqual(args, [
    "social",
    "post",
    "--accounts",
    "1,2",
    "--text",
    "hello",
    "--idempotency-key",
    "k1",
  ]);
});

test("known global --group still parsed", () => {
  const { flags, args } = parseArgs(["tokens", "create", "n", "--group", "default"]);
  assert.equal(flags.group, "default");
  assert.deepEqual(args, ["tokens", "create", "n"]);
});

test("double-dash stops flag parsing", () => {
  const { args } = parseArgs(["tokens", "exec", "1", "--", "echo", "--platform"]);
  assert.deepEqual(args, ["tokens", "exec", "1", "echo", "--platform"]);
});
