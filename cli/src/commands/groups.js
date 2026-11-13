/**
 * omnimux groups — list groups available to the current user, with ratios.
 * Endpoint: GET /api/user/self/groups
 */

const { printJson } = require("../lib/output");

async function run(ctx) {
  const data = await ctx.api("GET", "/api/user/self/groups");
  printJson(data, ctx.flags);
}

module.exports = { run };
