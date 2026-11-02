/**
 * Clipboard writer for the omnimux CLI.
 *
 * Ported from .agents/skills/newapi/scripts/copy-key.js: the secret is
 * written exclusively through a child-process pipe to the clipboard utility,
 * so it never appears on this process's stdout, stderr, or argv.
 */

const { execFileSync, spawnSync } = require("child_process");

/**
 * Detect an available clipboard utility.
 * @returns {{ cmd: string, args: string[] } | null}
 */
function detectClipboard() {
  if (process.platform === "darwin") return { cmd: "pbcopy", args: [] };
  if (process.platform === "win32") return { cmd: "clip", args: [] };
  for (const candidate of [
    { cmd: "xclip", args: ["-selection", "clipboard"] },
    { cmd: "xsel", args: ["--clipboard", "--input"] },
    { cmd: "wl-copy", args: [] },
  ]) {
    try {
      execFileSync("which", [candidate.cmd], { stdio: "ignore" });
      return candidate;
    } catch {}
  }
  return null;
}

/**
 * Copy a secret to the system clipboard via a child-process stdin pipe.
 * @param {string} secret
 * @throws {Error} generic message only — never contains the secret
 */
function copyToClipboard(secret) {
  const clip = detectClipboard();
  if (!clip) {
    throw new Error("No clipboard utility found (need pbcopy / clip / xclip / xsel / wl-copy)");
  }
  const r = spawnSync(clip.cmd, clip.args, {
    input: String(secret),
    stdio: ["pipe", "ignore", "ignore"],
  });
  if (r.error || r.status !== 0) {
    throw new Error("Failed to write to clipboard");
  }
}

module.exports = { detectClipboard, copyToClipboard };
