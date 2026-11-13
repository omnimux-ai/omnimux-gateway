const { test } = require("node:test");
const assert = require("node:assert/strict");

const { parseSocialFlags } = require("../src/commands/social");

test("parseSocialFlags: --file and --media", () => {
  const { flags } = parseSocialFlags([
    "--accounts",
    "1",
    "--text",
    "hi",
    "--media",
    "https://a/x.png,https://b/y.mp4",
    "--file",
    "./a.mp4",
    "--file",
    "./b.png",
  ]);
  assert.equal(flags.accounts, "1");
  assert.equal(flags.text, "hi");
  assert.equal(flags.media, "https://a/x.png,https://b/y.mp4");
  assert.deepEqual(flags.files, ["./a.mp4", "./b.png"]);
});

test("parseSocialFlags: --content-type", () => {
  const { flags, positional } = parseSocialFlags([
    "clip.mp4",
    "--content-type",
    "video/mp4",
  ]);
  assert.deepEqual(positional, ["clip.mp4"]);
  assert.equal(flags.contentType, "video/mp4");
});
