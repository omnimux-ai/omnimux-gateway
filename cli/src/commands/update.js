/**
 * omnimux update — self-update the CLI and migrate legacy base_url.
 *
 *   omnimux update           Check registry and install if newer
 *   omnimux update --check   Report only (no install)
 *   omnimux update --force   Reinstall latest even when versions match
 *
 * Alias: omnimux upgrade
 */

const PKG = require("../../package.json");
const { performUpdate } = require("../lib/update");
const { printJson, UsageError } = require("../lib/output");
const { ApiError } = require("../lib/errors");

async function run(ctx) {
  // Reject unexpected positional args (flags are global).
  if (ctx.args && ctx.args.length > 0) {
    throw new UsageError(
      "usage: omnimux update [--check] [--force]",
      { hint: "use --check to report only, or --force to reinstall" }
    );
  }

  // --dry-run is check-only (no install), same as --check for update.
  const checkOnly = !!ctx.flags.check || !!ctx.flags.dryRun || !!ctx.dryRun;

  const result = await performUpdate({
    currentVersion: PKG.version,
    checkOnly,
    force: !!ctx.flags.force && !checkOnly,
  });

  if (result && result.success === false) {
    throw new ApiError(
      String(result.message || result.error || "update failed"),
      { subtype: "upstream" }
    );
  }

  const payload =
    ctx.flags.dryRun || ctx.dryRun
      ? { dry_run: true, would: "install if outdated", ...result }
      : result;

  if (ctx.out) ctx.out.ok(payload);
  else printJson(payload, ctx.flags);
}

module.exports = { run };
