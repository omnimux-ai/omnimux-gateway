/**
 * omnimux pricing — public model price list (no auth token required).
 * Endpoint: GET /api/pricing
 */

const { printJson } = require("../lib/output");

async function run(ctx) {
  const data = await ctx.api("GET", "/api/pricing");
  printJson(data, ctx.flags);
}

module.exports = { run };
