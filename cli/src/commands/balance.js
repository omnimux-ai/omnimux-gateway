/**
 * omnimux balance — account balance and usage.
 * Endpoint: GET /api/user/self
 *
 * Adds derived USD convenience fields (raw quota units are 1/500000 USD).
 */

const { printJson } = require("../lib/output");

const QUOTA_PER_USD = 500000;

async function run(ctx) {
  const data = await ctx.api("GET", "/api/user/self");
  const self = data && data.data ? data.data : {};
  printJson(
    {
      success: data ? data.success !== false : true,
      data: self,
      derived: {
        quota_usd: typeof self.quota === "number" ? self.quota / QUOTA_PER_USD : null,
        used_quota_usd: typeof self.used_quota === "number" ? self.used_quota / QUOTA_PER_USD : null,
      },
    },
    ctx.flags
  );
}

module.exports = { run, QUOTA_PER_USD };
