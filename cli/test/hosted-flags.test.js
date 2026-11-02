const { test } = require("node:test");
const assert = require("node:assert/strict");

const {
  parseHostedFlags,
  parseChannelIds,
  parseKeywords,
  parseScheduledAt,
} = require("../src/commands/hosted");

test("parseHostedFlags: channel and channels flags", () => {
  const { flags: f1 } = parseHostedFlags(["post", "--channel", "12", "--caption", "Hello"]);
  assert.equal(f1.channel, "12");
  assert.equal(f1.caption, "Hello");

  const { flags: f2 } = parseHostedFlags(["batch", "--channels=1,2,3", "--caption=Matrix"]);
  assert.equal(f2.channels, "1,2,3");
  assert.equal(f2.caption, "Matrix");
});

test("parseHostedFlags: brand flags", () => {
  const { flags } = parseHostedFlags([
    "brand",
    "1",
    "--nickname=BrandX",
    "--avatar=https://cdn.com/a.png",
    "--bio=Official Bio",
    "--site=https://brand.com",
    "--keywords=ai,video,saas",
  ]);
  assert.equal(flags.nickname, "BrandX");
  assert.equal(flags.avatar, "https://cdn.com/a.png");
  assert.equal(flags.bio, "Official Bio");
  assert.equal(flags.site, "https://brand.com");
  assert.equal(flags.keywords, "ai,video,saas");
});

test("parseHostedFlags: order and subscription flags", () => {
  const { flags } = parseHostedFlags([
    "buy",
    "--sku=1",
    "--quantity=3",
    "--idempotency-key=buy-123",
    "--confirm",
  ]);
  assert.equal(flags.sku, "1");
  assert.equal(flags.quantity, "3");
  assert.equal(flags.idempotencyKey, "buy-123");
  assert.equal(flags.confirm, true);
});

test("parseHostedFlags: alias forms (-n, --nick-name, --avatar-url, --text, --schedule, --media-url)", () => {
  const { flags } = parseHostedFlags([
    "buy",
    "-n",
    "5",
    "--nick-name=AltNick",
    "--avatar-url=https://cdn.com/pic.jpg",
    "--text=AltCaption",
    "--schedule=1700000000",
    "--media-url=https://cdn.com/v.mp4",
  ]);
  assert.equal(flags.quantity, "5");
  assert.equal(flags.nickname, "AltNick");
  assert.equal(flags.avatar, "https://cdn.com/pic.jpg");
  assert.equal(flags.caption, "AltCaption");
  assert.equal(flags.scheduledFor, "1700000000");
  assert.equal(flags.media, "https://cdn.com/v.mp4");
});

test("parseChannelIds parses valid comma-separated ids and throws on invalid", () => {
  assert.deepEqual(parseChannelIds("1, 2,3"), [1, 2, 3]);
  assert.deepEqual(parseChannelIds("42"), [42]);
  assert.deepEqual(parseChannelIds(""), []);
  assert.throws(() => parseChannelIds("1,abc,3"), /invalid channel id/);
  assert.throws(() => parseChannelIds("0"), /invalid channel id/);
});

test("parseKeywords handles comma-separated list", () => {
  assert.deepEqual(parseKeywords("growth, viral, tiktok"), ["growth", "viral", "tiktok"]);
  assert.equal(parseKeywords(""), undefined);
});

test("parseScheduledAt handles unix timestamp and ISO strings", () => {
  assert.equal(parseScheduledAt(""), 0);
  assert.equal(parseScheduledAt("1700000000"), 1700000000);
  assert.equal(parseScheduledAt("1700000000000"), 1700000000);
  const epoch = Math.floor(new Date("2026-09-01T00:00:00Z").getTime() / 1000);
  assert.equal(parseScheduledAt("2026-09-01T00:00:00Z"), epoch);
  assert.throws(() => parseScheduledAt("not-a-date"), /invalid scheduled-for time/);
});
