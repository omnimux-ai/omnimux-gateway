/**
 * omnimux scan-config <file> — inspect a config file's structure with
 * best-effort secret redaction (ported from inject-key.js --scan).
 *
 * Best-effort, not a formal parser: treats redaction as risk reduction,
 * not an absolute guarantee. Default output is JSON ({ path, content });
 * --pretty prints the sanitized content verbatim.
 */

const fs = require("fs");
const path = require("path");
const { sanitize } = require("../lib/mask");
const { printJson, UsageError } = require("../lib/output");

async function run(ctx) {
  const filePath = ctx.args[0];
  if (!filePath) {
    throw new UsageError("usage: omnimux scan-config <file>");
  }
  const resolved = path.resolve(filePath);
  if (!fs.existsSync(resolved)) {
    throw new UsageError(`file not found: ${resolved}`);
  }
  const content = sanitize(fs.readFileSync(resolved, "utf-8"));
  if (ctx.flags.pretty) {
    process.stdout.write(content + (content.endsWith("\n") ? "" : "\n"));
  } else {
    printJson({ path: resolved, content, redaction: "best-effort" }, ctx.flags);
  }
}

module.exports = { run };
