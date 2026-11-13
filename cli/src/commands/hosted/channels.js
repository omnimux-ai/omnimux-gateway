const { UsageError } = require("../../lib/output");
const { parseKeywords } = require("./flags");
const { unwrapApiData } = require("./shared");

async function handleStatus(ctx) {
  const res = await ctx.api("GET", "/api/hosted/v1/status");
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleSkus(ctx) {
  const res = await ctx.api("GET", "/api/hosted/v1/skus");
  const data = unwrapApiData(res);
  ctx.out.ok({ skus: Array.isArray(data) ? data : [] });
}

async function handleChannels(ctx, flags) {
  let url = "/api/hosted/v1/channels";
  if (flags.status) {
    url += `?status=${encodeURIComponent(flags.status)}`;
  }
  const res = await ctx.api("GET", url);
  const data = unwrapApiData(res);
  ctx.out.ok({ channels: Array.isArray(data) ? data : [] });
}

async function handleChannel(ctx, positional, flags) {
  const id = positional[1] || flags.channel;
  if (!id) {
    throw new UsageError("channel id is required (e.g. omnimux hosted channel 1)");
  }
  const res = await ctx.api("GET", `/api/hosted/v1/channels/${encodeURIComponent(id)}`);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleBrand(ctx, positional, flags) {
  const id = positional[1] || flags.channel;
  if (!id) {
    throw new UsageError("channel id is required (e.g. omnimux hosted brand 1 --bio='...')");
  }
  const brandBody = {};
  if (flags.nickname !== undefined) brandBody.nick_name = flags.nickname;
  if (flags.avatar !== undefined) brandBody.avatar_url = flags.avatar;
  if (flags.bio !== undefined) brandBody.bio = flags.bio;
  if (flags.site !== undefined) brandBody.site = flags.site;
  const kw = parseKeywords(flags.keywords);
  if (kw !== undefined) brandBody.keywords = kw;

  if (Object.keys(brandBody).length === 0) {
    throw new UsageError("at least one brand field (--nickname, --avatar, --bio, --site, --keywords) is required");
  }

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.brand",
      channel_id: Number(id),
      brand: brandBody,
    });
    return;
  }

  const res = await ctx.api("PATCH", `/api/hosted/v1/channels/${encodeURIComponent(id)}/brand`, brandBody);
  const data = unwrapApiData(res);
  ctx.out.ok({ channel_id: Number(id), ...data });
}

module.exports = {
  handleStatus,
  handleSkus,
  handleChannels,
  handleChannel,
  handleBrand,
};
