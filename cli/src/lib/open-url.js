/**
 * Cross-platform URL opener for interactive CLI flows.
 * Failure is non-fatal — callers should still print the URL.
 */

const { spawnSync } = require("child_process");

/**
 * Validate and normalize an external URL before passing it to an OS opener.
 * Only http(s) URLs without control characters are allowed.
 * @param {string} rawUrl
 * @returns {{ ok: true, url: string } | { ok: false, error: string }}
 */
function validateOpenUrl(rawUrl) {
  if (!rawUrl || typeof rawUrl !== "string") {
    return { ok: false, error: "missing url" };
  }
  const trimmed = rawUrl.trim();
  if (!trimmed || /[\u0000-\u001f\u007f]/.test(trimmed)) {
    return { ok: false, error: "invalid url" };
  }
  let parsed;
  try {
    parsed = new URL(trimmed);
  } catch {
    return { ok: false, error: "invalid url" };
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return { ok: false, error: "unsupported url protocol" };
  }
  const normalized = parsed.toString();
  if (/[\u0000-\u001f\u007f]/.test(normalized)) {
    return { ok: false, error: "invalid url" };
  }
  return { ok: true, url: normalized };
}

/**
 * @param {string} url
 * @returns {{ ok: boolean, error?: string }}
 */
function openUrl(url) {
  const checked = validateOpenUrl(url);
  if (!checked.ok) {
    return { ok: false, error: checked.error };
  }
  const safeUrl = checked.url;
  try {
    let result;
    if (process.platform === "darwin") {
      result = spawnSync("open", ["--", safeUrl], { stdio: "ignore" });
    } else if (process.platform === "win32") {
      result = spawnSync("rundll32.exe", ["url.dll,FileProtocolHandler", safeUrl], {
        stdio: "ignore",
        windowsHide: true,
      });
    } else {
      result = spawnSync("xdg-open", [safeUrl], { stdio: "ignore" });
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

module.exports = { openUrl, validateOpenUrl };
