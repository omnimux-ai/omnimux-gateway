const { test } = require("node:test");
const assert = require("node:assert/strict");

const { parseSocialFlags, buildTikTokOptions } = require("../src/commands/social");

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

test("parseSocialFlags + buildTikTokOptions: photo draft and compliance flags preserve 0 and false", () => {
  const { flags } = parseSocialFlags([
    "--title=Slide Title",
    "--description",
    "Slide Desc",
    "--photo-cover-index=0",
    "--is-aigc",
    "false",
    "--privacy-level=FOLLOWER_OF_CREATOR",
    "--allow-comment=true",
    "--no-allow-duet",
    "--allow-stitch=false",
    "--commercial-content",
    "--your-brand=true",
    "--branded-content=false",
    "--music-usage-confirmed=true",
  ]);
  assert.deepEqual(buildTikTokOptions(flags), {
    title: "Slide Title",
    description: "Slide Desc",
    photo_cover_index: 0,
    is_aigc: false,
    privacy_level: "FOLLOWER_OF_CREATOR",
    allow_comment: true,
    allow_duet: false,
    allow_stitch: false,
    commercial_content: true,
    your_brand: true,
    branded_content: false,
    music_usage_confirmed: true,
  });
});

test("parseSocialFlags + buildTikTokOptions: rejects invalid cover index, boolean, or JSON", () => {
  assert.throws(() => parseSocialFlags(["--photo-cover-index", "-1"]), /non-negative integer/);
  assert.throws(() => parseSocialFlags(["--photo-cover-index=abc"]), /non-negative integer/);
  assert.throws(() => parseSocialFlags(["--is-aigc=maybe"]), /boolean/);
  assert.throws(
    () => buildTikTokOptions({ tiktokOptionsJson: "not-json" }),
    /--tiktok-options/
  );
  assert.throws(
    () => buildTikTokOptions({ tiktokOptionsJson: "[1,2]" }),
    /--tiktok-options/
  );
  assert.throws(
    () => buildTikTokOptions({ tiktokOptionsJson: '{"photo_cover_index":-1}' }),
    /photo_cover_index/
  );
});
