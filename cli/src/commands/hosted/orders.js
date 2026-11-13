const { UsageError } = require("../../lib/output");
const { parseKeywords } = require("./flags");
const { unwrapApiData } = require("./shared");

async function handleOrder(ctx, positional) {
  const orderId = positional[1];
  if (!orderId) {
    throw new UsageError("order id is required (e.g. omnimux hosted order 1)");
  }
  const res = await ctx.api("GET", `/api/hosted/v1/orders/${encodeURIComponent(orderId)}`);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleBuy(ctx, positional, flags) {
  const skuId = Number(flags.sku || positional[1] || 1);
  const qty = Number(flags.quantity || 1);
  if (!Number.isInteger(qty) || qty <= 0) {
    throw new UsageError("quantity must be a positive integer (--quantity=<n>)");
  }

  const orderBody = {
    sku_id: skuId,
    quantity: qty,
    idempotency_key: flags.idempotencyKey || "",
    brand: {
      nick_name: flags.nickname || "",
      avatar_url: flags.avatar || "",
      bio: flags.bio || "",
      site: flags.site || "",
      keywords: parseKeywords(flags.keywords) || [],
    },
  };

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.buy",
      sku_id: skuId,
      quantity: qty,
      estimated_amount_usd: qty * 99,
      order: orderBody,
    });
    return;
  }

  const res = await ctx.api("POST", "/api/hosted/v1/orders", orderBody);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleAddon(ctx, positional, flags) {
  const skuId = Number(flags.sku || positional[1] || 1);
  const qty = Number(flags.quantity || 1);
  if (!Number.isInteger(qty) || qty <= 0) {
    throw new UsageError("quantity must be a positive integer (--quantity=<n>)");
  }

  const addonBody = {
    sku_id: skuId,
    quantity: qty,
    confirm: Boolean(flags.confirm),
    idempotency_key: flags.idempotencyKey || "",
  };

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.addon",
      sku_id: skuId,
      quantity: qty,
      confirm: Boolean(flags.confirm),
    });
    return;
  }

  const res = await ctx.api("POST", `/api/hosted/v1/subscriptions/${encodeURIComponent(skuId)}/addons`, addonBody);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleRenew(ctx, positional, flags) {
  const skuId = Number(flags.sku || positional[1] || 1);
  const renewBody = {
    sku_id: skuId,
    idempotency_key: flags.idempotencyKey || "",
  };

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.renew",
      sku_id: skuId,
    });
    return;
  }

  const res = await ctx.api("POST", `/api/hosted/v1/subscriptions/${encodeURIComponent(skuId)}/renew`, renewBody);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

module.exports = {
  handleOrder,
  handleBuy,
  handleAddon,
  handleRenew,
};
