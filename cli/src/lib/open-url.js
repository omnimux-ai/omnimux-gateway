/**
 * Cross-platform URL opener for interactive CLI flows.
 * Failure is non-fatal — callers should still print the URL.
 */

const { spawnSync } = require("child_process");

/**
 * @param {string} url
 * @returns {{ ok: boolean, error?: string }}
 */
function openUrl(url) {
  if (!url || typeof url !== "string") {
    return { ok: false, error: "missing url" };
  }
  try {
    let result;
    if (process.platform === "darwin") {
      result = spawnSync("open", [url], { stdio: "ignore" });
    } else if (process.platform === "win32") {
      result = spawnSync("cmd", ["/c", "start", "", url], {
        stdio: "ignore",
        windowsHide: true,
      });
    } else {
      result = spawnSync("xdg-open", [url], { stdio: "ignore" });
    }
    if (result && result.error) {
      return { ok: false, error: String(result.error.message || result.error) };
    }
    if (result && result.status !== 0 && result.status != null) {
      return { ok: false, error: `exit ${result.status}` };
    }
    return { ok: true };
  } catch (err) {
    return { ok: false, error: err && err.message ? err.message : String(err) };
  }
}

module.exports = { openUrl };
