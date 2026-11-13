const { UsageError } = require("../../lib/output");

function parseHostedFlags(args) {
  const flags = { files: [], channelIds: [] };
  const positional = [];
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === "--channel") {
      flags.channel = args[++i];
    } else if (a.startsWith("--channel=")) {
      flags.channel = a.slice("--channel=".length);
    } else if (a === "--channels") {
      flags.channels = args[++i];
    } else if (a.startsWith("--channels=")) {
      flags.channels = a.slice("--channels=".length);
    } else if (a === "--sku") {
      flags.sku = args[++i];
    } else if (a.startsWith("--sku=")) {
      flags.sku = a.slice("--sku=".length);
    } else if (a === "--quantity" || a === "-n") {
      flags.quantity = args[++i];
    } else if (a.startsWith("--quantity=")) {
      flags.quantity = a.slice("--quantity=".length);
    } else if (a === "--status") {
      flags.status = args[++i];
    } else if (a.startsWith("--status=")) {
      flags.status = a.slice("--status=".length);
    } else if (a === "--nickname" || a === "--nick-name") {
      flags.nickname = args[++i];
    } else if (a.startsWith("--nickname=") || a.startsWith("--nick-name=")) {
      flags.nickname = a.split("=")[1];
    } else if (a === "--avatar" || a === "--avatar-url") {
      flags.avatar = args[++i];
    } else if (a.startsWith("--avatar=") || a.startsWith("--avatar-url=")) {
      flags.avatar = a.split("=")[1];
    } else if (a === "--bio") {
      flags.bio = args[++i];
    } else if (a.startsWith("--bio=")) {
      flags.bio = a.slice("--bio=".length);
    } else if (a === "--site") {
      flags.site = args[++i];
    } else if (a.startsWith("--site=")) {
      flags.site = a.slice("--site=".length);
    } else if (a === "--keywords") {
      flags.keywords = args[++i];
    } else if (a.startsWith("--keywords=")) {
      flags.keywords = a.slice("--keywords=".length);
    } else if (a === "--media" || a === "--media-url") {
      flags.media = args[++i];
    } else if (a.startsWith("--media=") || a.startsWith("--media-url=")) {
      flags.media = a.split("=")[1];
    } else if (a === "--file") {
      flags.files.push(args[++i]);
    } else if (a.startsWith("--file=")) {
      flags.files.push(a.slice("--file=".length));
    } else if (a === "--caption" || a === "--text") {
      flags.caption = args[++i];
    } else if (a.startsWith("--caption=") || a.startsWith("--text=")) {
      flags.caption = a.split("=")[1];
    } else if (a === "--scheduled-for" || a === "--schedule") {
      flags.scheduledFor = args[++i];
    } else if (a.startsWith("--scheduled-for=") || a.startsWith("--schedule=")) {
      flags.scheduledFor = a.split("=")[1];
    } else if (a === "--idempotency-key") {
      flags.idempotencyKey = args[++i];
    } else if (a.startsWith("--idempotency-key=")) {
      flags.idempotencyKey = a.slice("--idempotency-key=".length);
    } else if (a === "--content-type") {
      flags.contentType = args[++i];
    } else if (a.startsWith("--content-type=")) {
      flags.contentType = a.slice("--content-type=".length);
    } else if (a === "--confirm") {
      flags.confirm = true;
    } else if (a.startsWith("--")) {
      throw new UsageError(`unknown hosted flag: ${a}`);
    } else {
      positional.push(a);
    }
  }
  return { flags, positional };
}

function parseChannelIds(raw) {
  if (!raw) return [];
  return String(raw)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .map((s) => {
      const n = Number(s);
      if (!Number.isInteger(n) || n <= 0) {
        throw new UsageError(`invalid channel id: ${s}`);
      }
      return n;
    });
}

function parseKeywords(raw) {
  if (!raw) return undefined;
  return String(raw)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function parseScheduledAt(raw) {
  if (!raw) return 0;
  const num = Number(raw);
  if (!Number.isNaN(num) && num > 0) {
    return Math.floor(num > 1e11 ? num / 1000 : num);
  }
  const ms = Date.parse(raw);
  if (Number.isNaN(ms) || ms <= 0) {
    throw new UsageError(`invalid scheduled-for time: ${raw}`);
  }
  return Math.floor(ms / 1000);
}

module.exports = {
  parseHostedFlags,
  parseChannelIds,
  parseKeywords,
  parseScheduledAt,
};
