/**
 * Placeholder injection for config files.
 *
 * Ported from .agents/skills/newapi/scripts/inject-key.js: replaces the
 * `__OMNIMUX_TOKEN_{id}__` placeholder with the real key and atomically
 * replaces the target file. The key never touches stdout or stderr.
 */

const fs = require("fs");
const path = require("path");

function placeholderFor(tokenId) {
  return `__OMNIMUX_TOKEN_${tokenId}__`;
}

function buildTempPath(targetPath) {
  const dir = path.dirname(targetPath);
  const base = path.basename(targetPath);
  return path.join(dir, `.${base}.omnimux-${process.pid}-${Date.now()}.tmp`);
}

function writeFileAtomically(targetPath, content) {
  const tempPath = buildTempPath(targetPath);
  const stats = fs.statSync(targetPath);

  try {
    fs.writeFileSync(tempPath, content, { encoding: "utf-8", mode: stats.mode });
    fs.renameSync(tempPath, targetPath);
  } catch {
    try {
      if (fs.existsSync(tempPath)) {
        fs.unlinkSync(tempPath);
      }
    } catch {}

    throw new Error(
      `Failed to safely replace ${targetPath}. The original file was left untouched.`
    );
  }
}

/**
 * Replace the token placeholder in `filePath` with `secret`.
 * @param {string} filePath
 * @param {string|number} tokenId
 * @param {string} secret real key — used in memory only
 * @throws {Error} generic messages only — never contain the secret
 */
function injectPlaceholder(filePath, tokenId, secret) {
  const resolved = path.resolve(filePath);
  if (!fs.existsSync(resolved)) {
    throw new Error(`File not found: ${resolved}`);
  }

  const placeholder = placeholderFor(tokenId);
  const content = fs.readFileSync(resolved, "utf-8");
  if (!content.includes(placeholder)) {
    throw new Error(`Placeholder ${placeholder} not found in ${filePath}`);
  }

  const updated = content.split(placeholder).join(String(secret));
  writeFileAtomically(resolved, updated);
}

module.exports = { placeholderFor, injectPlaceholder, writeFileAtomically };
