/**
 * Secret masking for the omnimux CLI.
 *
 * Ported from .agents/skills/newapi/scripts/sanitize.js (text redaction) and
 * the field-masking walk in .agents/skills/newapi/scripts/api.js.
 *
 * Hard rule: a full `sk-` key must never reach stdout, stderr, logs, or error
 * messages. Everything user-visible goes through these helpers.
 */

const SENSITIVE_KEYWORDS = [
  "password", "passwd", "secret", "token", "credential",
  "apikey", "api_key", "api-key", "api_secret",
  "auth", "auth_token", "authorization",
  "private_key", "private-key", "privatekey",
  "access_key", "access-key", "accesskey",
  "client_secret", "client-secret",
];

const SENSITIVE_PATTERN = new RegExp(
  "(" + SENSITIVE_KEYWORDS.join("|") + ")",
  "i"
);

/** Do not retain bearer-style media capabilities in logs, errors, or previews. */
function sanitizeMediaUrls(content) {
  return String(content).replace(/https?:\/\/[^\s"'<>]+|\/(?:v1\/tasks|api\/task|api\/influencer\/v1\/tasks)\/[^\s"'<>]+/gi, (value) => {
    let url;
    try { url = new URL(value, "https://redaction.invalid"); } catch { return value; }
    const signed = [...url.searchParams.keys()].some((key) => /^(?:access|access_token|token|signature|sig|credential|x-amz-signature|x-amz-credential|x-goog-signature|x-goog-credential)$/i.test(key));
    return signed ? "[PRIVATE_MEDIA_URL]" : value;
  });
}

/**
 * Best-effort redaction of sensitive values in arbitrary text
 * (JSON / YAML / ENV / TOML-ish content). Not a formal parser.
 */
function sanitize(content) {
  // Rule 1: sk- prefixed tokens
  let result = sanitizeMediaUrls(content).replace(/(?<![A-Za-z0-9_-])sk-[A-Za-z0-9_\-]{4,}/g, "sk-<REDACTED>");

  // Rule 2: Bearer tokens
  result = result.replace(/Bearer\s+[A-Za-z0-9_.\-\/+=]{4,}/g, "Bearer <REDACTED>");

  // Rule 3: Credentials in connection strings (user:pass@host pattern)
  result = result.replace(
    /[A-Za-z0-9_.\-]+:[A-Za-z0-9_.\-]+@[^\s]+/g,
    "<REDACTED>"
  );

  // Rule 4: Values of sensitive-named fields (line-by-line)
  result = result
    .split("\n")
    .map((line) => {
      // JSON: "key": "value"  or  "key": "value",
      const jsonMatch = line.match(
        /^(\s*"([^"]+)"\s*:\s*)"([^"]*)"(.*)$/
      );
      if (jsonMatch) {
        const [, prefix, key, , suffix] = jsonMatch;
        if (SENSITIVE_PATTERN.test(key)) {
          return `${prefix}"<REDACTED>"${suffix}`;
        }
        return line;
      }

      // YAML: key: value  (unquoted or quoted)
      const yamlMatch = line.match(
        /^(\s*([\w.\-]+)\s*:\s*)(.+)$/
      );
      if (yamlMatch) {
        const [, prefix, key, value] = yamlMatch;
        if (SENSITIVE_PATTERN.test(key) && value.trim() !== "" && value.trim() !== "|" && value.trim() !== ">") {
          return `${prefix}<REDACTED>`;
        }
        return line;
      }

      // ENV / TOML: KEY=value  or  KEY = "value"
      const envMatch = line.match(
        /^(\s*([\w.\-]+)\s*=\s*)(.+)$/
      );
      if (envMatch) {
        const [, prefix, key] = envMatch;
        if (SENSITIVE_PATTERN.test(key)) {
          return `${prefix}<REDACTED>`;
        }
        return line;
      }

      return line;
    })
    .join("\n");

  return result;
}

// --- JSON response field masking (ported from api.js) ---

const SENSITIVE_KEYS = new Set([
  "key",
  "keys",
  "secret",
  "password",
  "access_token",
  "client_secret",
  "api_key",
  "apikey",
]);

/**
 * Mask a single token-key string, preserving the server-side masking shape:
 * first 4 + ********** + last 4 for long keys. Idempotent for values the
 * server already masked (same algorithm), so GET -> PUT round-trips of token
 * objects keep the masked key intact.
 */
function maskTokenKey(value) {
  if (typeof value !== "string") return value;
  const raw = value.startsWith("sk-") ? value.slice(3) : value;
  if (!raw) return value;
  const prefix = value.startsWith("sk-") ? "sk-" : "";
  if (raw.length <= 4) return prefix + "*".repeat(raw.length);
  if (raw.length <= 8) return prefix + raw.slice(0, 2) + "****" + raw.slice(-2);
  return prefix + raw.slice(0, 4) + "**********" + raw.slice(-4);
}

/**
 * Deep-walk a parsed JSON value and mask sensitive-named fields.
 * Applied to API paths that may carry keys (/api/token, /api/user).
 */
function maskSensitiveJson(node) {
  if (Array.isArray(node)) return node.map(maskSensitiveJson);
  if (node !== null && typeof node === "object") {
    const masked = {};
    for (const [k, v] of Object.entries(node)) {
      const lower = k.toLowerCase();
      if (SENSITIVE_KEYS.has(lower) && typeof v === "string") {
        masked[k] = maskTokenKey(v);
      } else if (SENSITIVE_KEYS.has(lower) && Array.isArray(v)) {
        masked[k] = v.map((item) =>
          typeof item === "string" ? maskTokenKey(item) : maskSensitiveJson(item)
        );
      } else {
        masked[k] = maskSensitiveJson(v);
      }
    }
    return masked;
  }
  return node;
}

/** True for user-level API paths whose responses may contain keys. */
function isSensitivePath(urlPath) {
  return (
    urlPath.startsWith("/api/token") ||
    urlPath.startsWith("/api/user")
  );
}

module.exports = {
  sanitize,
  sanitizeMediaUrls,
  maskTokenKey,
  maskSensitiveJson,
  isSensitivePath,
  SENSITIVE_KEYWORDS,
  SENSITIVE_PATTERN,
};
