/**
 * Side-effect gates: dry-run support + high-risk confirmation.
 *
 * Called after leaf resolution and before auto-update / auth / handlers.
 */

const { getLeaf } = require("./registry");
const { UsageError, ConfirmationRequiredError } = require("./errors");

/**
 * @param {string} leafName registry leaf name (e.g. tokens.apply)
 * @param {{ dryRun?: boolean, yes?: boolean }} flags
 * @param {string[]} [rawArgv] original argv for recovery suggestion
 */
function enforceRiskGate(leafName, flags = {}, rawArgv = []) {
  const leaf = getLeaf(leafName);
  // Unknown leaves: no gate (top-level without registry entry still runs)
  if (!leaf) return { leaf: null, dryRun: Boolean(flags.dryRun) };

  const dryRun = Boolean(flags.dryRun);
  const yes = Boolean(flags.yes);

  if (dryRun && !leaf.dryRun) {
    throw new UsageError(`--dry-run is not supported for ${leaf.name}`, {
      subtype: "dry_run_unsupported",
      param: "--dry-run",
      hint: "omit --dry-run to execute, or use a command that supports dry-run (see omnimux schema)",
      recovery: { command: "omnimux", args: ["schema", leaf.name] },
    });
  }

  // update --check is read-only version report; not a high-risk install.
  const updateCheckOnly = leaf.name === "update" && Boolean(flags.check);

  if (leaf.risk === "high-risk-write" && !yes && !dryRun && !updateCheckOnly) {
    const recoveryArgs = buildYesRecoveryArgs(rawArgv);
    throw new ConfirmationRequiredError(
      `${leaf.name} requires confirmation`,
      {
        action: leaf.name,
        risk: "high-risk-write",
        hint: "add --yes after explicit user approval (or --dry-run to preview)",
        recovery: { command: "omnimux", args: recoveryArgs },
      }
    );
  }

  return { leaf, dryRun };
}

/**
 * Rebuild argv with --yes appended (preserve user args).
 * @param {string[]} rawArgv process.argv.slice(2)
 */
function buildYesRecoveryArgs(rawArgv) {
  const args = Array.isArray(rawArgv) ? rawArgv.filter((a) => a !== "--yes") : [];
  if (!args.includes("--yes")) args.push("--yes");
  return args;
}

module.exports = {
  enforceRiskGate,
  buildYesRecoveryArgs,
};
