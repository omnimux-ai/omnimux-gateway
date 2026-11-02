/**
 * Hosted Distribution Channels commands (GeeLark-backed via OmniMux Hosted API).
 *
 * Routes: /api/hosted/v1/*
 * User-level CLI only: does not talk to GeeLark directly, no admin endpoints.
 */

const { UsageError } = require("../../lib/output");
const {
  parseHostedFlags,
  parseChannelIds,
  parseKeywords,
  parseScheduledAt,
} = require("./flags");

const {
  handleStatus,
  handleSkus,
  handleChannels,
  handleChannel,
  handleBrand,
} = require("./channels");

const {
  handleUpload,
  handlePost,
  handleBatch,
  handleTasks,
  handleTask,
  handleCancel,
} = require("./tasks");

const {
  handleOrder,
  handleBuy,
  handleAddon,
  handleRenew,
} = require("./orders");

const HANDLERS = {
  status: (ctx) => handleStatus(ctx),
  skus: (ctx) => handleSkus(ctx),
  channels: (ctx, positional, flags) => handleChannels(ctx, flags),
  channel: (ctx, positional, flags) => handleChannel(ctx, positional, flags),
  brand: (ctx, positional, flags) => handleBrand(ctx, positional, flags),
  upload: (ctx, positional, flags) => handleUpload(ctx, positional, flags),
  post: (ctx, positional, flags) => handlePost(ctx, positional, flags),
  batch: (ctx, positional, flags) => handleBatch(ctx, flags),
  tasks: (ctx, positional, flags) => handleTasks(ctx, positional, flags),
  task: (ctx, positional) => handleTask(ctx, positional),
  cancel: (ctx, positional) => handleCancel(ctx, positional),
  order: (ctx, positional) => handleOrder(ctx, positional),
  buy: (ctx, positional, flags) => handleBuy(ctx, positional, flags),
  addon: (ctx, positional, flags) => handleAddon(ctx, positional, flags),
  renew: (ctx, positional, flags) => handleRenew(ctx, positional, flags),
};

async function run(ctx) {
  const { flags, positional } = parseHostedFlags(ctx.args);
  const sub = positional[0] || "status";

  const handler = HANDLERS[sub];
  if (!handler) {
    throw new UsageError(`unknown hosted subcommand: ${sub} (see \`omnimux schema hosted\` or \`omnimux help\`)`);
  }

  await handler(ctx, positional, flags);
}

module.exports = {
  run,
  parseHostedFlags,
  parseChannelIds,
  parseKeywords,
  parseScheduledAt,
};
