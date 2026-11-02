/**
 * Optional meta.notices for Agent recovery (update / skills drift).
 *
 * Suppress:
 *   OMNIMUX_NO_UPDATE_NOTIFIER=1
 *   OMNIMUX_NO_SKILLS_NOTIFIER=1
 */

const PKG = require("../../package.json");
const { skillStatus } = require("./skill");
const { loadUpdateState, stripVersionPrefix, compareSemver } = require("./update");

function envTruthy(name) {
  const v = process.env[name];
  if (v == null || v === "") return false;
  return ["1", "true", "yes", "on"].includes(String(v).trim().toLowerCase());
}

/**
 * @param {{
 *   cliVersion?: string,
 *   autoUpdateResult?: object|null,
 *   cwd?: string,
 * }} [opts]
 * @returns {object|undefined}
 */
function composeNotices(opts = {}) {
  /** @type {Record<string, object>} */
  const notices = {};
  const cliVersion = stripVersionPrefix(opts.cliVersion || PKG.version);

  if (!envTruthy("OMNIMUX_NO_UPDATE_NOTIFIER")) {
    const updateNotice = buildUpdateNotice(cliVersion, opts.autoUpdateResult || null);
    if (updateNotice) notices.update = updateNotice;
  }

  if (!envTruthy("OMNIMUX_NO_SKILLS_NOTIFIER")) {
    const skillsNotice = buildSkillsNotice(opts.cwd || process.cwd(), cliVersion);
    if (skillsNotice) notices.skills = skillsNotice;
  }

  return Object.keys(notices).length ? notices : undefined;
}

/**
 * @param {string} cliVersion
 * @param {object|null} autoUpdateResult
 */
function buildUpdateNotice(cliVersion, autoUpdateResult) {
  // Prefer live auto-update outcome from this process.
  if (autoUpdateResult && !autoUpdateResult.skipped) {
    if (autoUpdateResult.action === "updated") {
      return {
        current: stripVersionPrefix(autoUpdateResult.previous_version || cliVersion),
        latest: stripVersionPrefix(autoUpdateResult.latest_version || autoUpdateResult.current_version || ""),
        message: "CLI was auto-updated; subsequent runs use the new version",
        recovery: { command: "omnimux", args: ["update", "--check"] },
      };
    }
    if (
      autoUpdateResult.action === "check_only" &&
      autoUpdateResult.latest_version &&
      compareSemver(
        stripVersionPrefix(autoUpdateResult.latest_version),
        stripVersionPrefix(autoUpdateResult.current_version || cliVersion)
      ) > 0
    ) {
      return {
        current: stripVersionPrefix(autoUpdateResult.current_version || cliVersion),
        latest: stripVersionPrefix(autoUpdateResult.latest_version),
        message: "A newer CLI version is available",
        recovery: { command: "omnimux", args: ["update", "--yes"] },
      };
    }
  }

  // Fallback: last persisted state (throttled runs still surface stale "updated" noise avoided).
  try {
    const st = loadUpdateState();
    if (st.last_result === "updated" && st.last_version) {
      const latest = stripVersionPrefix(st.last_version);
      if (compareSemver(latest, cliVersion) > 0) {
        return {
          current: cliVersion,
          latest,
          message: "A newer CLI version was reported; run update",
          recovery: { command: "omnimux", args: ["update", "--yes"] },
        };
      }
    }
  } catch {
    /* ignore */
  }
  return null;
}

/**
 * @param {string} cwd
 * @param {string} cliVersion
 */
function buildSkillsNotice(cwd, cliVersion) {
  let status;
  try {
    status = skillStatus({ cwd });
  } catch {
    return null;
  }
  if (!status || !Array.isArray(status.drifts) || status.drifts.length === 0) {
    return null;
  }
  return {
    drift: true,
    count: status.drifts.length,
    current: cliVersion,
    message: status.recommendation || "Installed skills are out of sync with this CLI",
    recovery: { command: "omnimux", args: ["skill", "update"] },
  };
}

module.exports = {
  composeNotices,
  buildUpdateNotice,
  buildSkillsNotice,
  envTruthy,
};
