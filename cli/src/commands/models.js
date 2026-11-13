/**
 * omnimux models — list models available to the current user.
 * Endpoint: GET /api/user/models
 */

const { apiRequest } = require("../lib/http");
const { printJson } = require("../lib/output");

async function run(ctx) {
  const data = await ctx.api("GET", "/api/user/models");
  printJson(data, ctx.flags);
}

module.exports = { run };
