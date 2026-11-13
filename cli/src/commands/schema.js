/**
 * omnimux schema — machine-readable command catalog for Agents.
 */

const { schemaPayload } = require("../lib/registry");
const { UsageError } = require("../lib/errors");

async function run(ctx) {
  const name = (ctx.args && ctx.args[0]) || "";
  const payload = schemaPayload(name || undefined);
  if (name && !payload) {
    throw new UsageError(`unknown schema leaf: ${name}`, {
      subtype: "unknown_command",
      hint: "run omnimux schema for the full catalog",
      recovery: { command: "omnimux", args: ["schema"] },
    });
  }
  ctx.out.ok(payload);
}

module.exports = {
  run,
  meta: {
    name: "schema",
    auth: "none",
    risk: "read",
  },
};
