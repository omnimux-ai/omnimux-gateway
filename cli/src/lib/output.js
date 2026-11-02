/**
 * Output contract for the omnimux CLI (Agent RPC).
 *
 * - Success: stdout JSON `{ ok: true, data, meta }` (stderr must stay empty).
 * - Error:   stderr JSON `{ ok: false, error, meta }` + non-zero exit.
 * - Exit: 0 ok, 1 api, 2 config/auth, 3 usage, 4 network, 5 internal, 10 confirmation.
 * - tokens exec post-spawn is a deliberate exception (see ERROR_CONTRACT.md).
 */

const { sanitize } = require("./mask");
const {
  CONTRACT_VERSION,
  exitCodeOf,
  asCliError,
  toWireError,
  UsageError,
} = require("./errors");

/** @type {{ command: string, cli_version: string, contract_version: number, notices?: object }} */
let activeMeta = {
  command: "unknown",
  cli_version: "0.0.0",
  contract_version: CONTRACT_VERSION,
};

/**
 * Set meta defaults for the current invocation (called from bin.js).
 * @param {{ command: string, cli_version: string, notices?: object }} meta
 */
function setActiveMeta(meta) {
  activeMeta = {
    command: meta.command || "unknown",
    cli_version: meta.cli_version || "0.0.0",
    contract_version: CONTRACT_VERSION,
  };
  if (meta.notices && typeof meta.notices === "object") {
    activeMeta.notices = meta.notices;
  }
}

function getActiveMeta() {
  return { ...activeMeta };
}

/**
 * Build success meta (allowlist).
 * @param {object} [extra]
 */
function buildMeta(extra) {
  const meta = {
    command: activeMeta.command,
    cli_version: activeMeta.cli_version,
    contract_version: CONTRACT_VERSION,
  };
  if (activeMeta.notices) meta.notices = activeMeta.notices;
  if (extra && typeof extra === "object") {
    for (const [k, v] of Object.entries(extra)) {
      if (v !== undefined) meta[k] = v;
    }
  }
  return meta;
}

function writeJson(stream, value, pretty) {
  const text = pretty ? JSON.stringify(value, null, 2) : JSON.stringify(value);
  stream.write(text + "\n");
}

/**
 * Deep-sanitize string leaves in a JSON-serializable value.
 * @param {unknown} value
 * @returns {unknown}
 */
function sanitizeDeep(value) {
  if (typeof value === "string") return sanitize(value);
  if (Array.isArray(value)) return value.map(sanitizeDeep);
  if (value && typeof value === "object") {
    /** @type {Record<string, unknown>} */
    const out = {};
    for (const [k, v] of Object.entries(value)) {
      out[k] = sanitizeDeep(v);
    }
    return out;
  }
  return value;
}

/**
 * Success envelope to stdout.
 * @param {unknown} data
 * @param {{ pretty?: boolean, meta?: object }} [opts]
 */
function ok(data, opts = {}) {
  const pretty = Boolean(opts.pretty);
  const envelope = {
    ok: true,
    data: sanitizeDeep(data === undefined ? null : data),
    meta: buildMeta(opts.meta),
  };
  writeJson(process.stdout, envelope, pretty);
}

/**
 * Error envelope to stderr; returns exit code.
 * Never throws; never leaks secrets.
 * @param {unknown} err
 * @param {{ pretty?: boolean, meta?: object }} [opts]
 * @returns {number}
 */
function fail(err, opts = {}) {
  const pretty = Boolean(opts.pretty);
  const typed = asCliError(err);
  const wireError = toWireError(typed);
  // Sanitize message/hint after typing
  if (typeof wireError.message === "string") {
    wireError.message = sanitize(wireError.message);
  }
  if (typeof wireError.hint === "string") {
    wireError.hint = sanitize(wireError.hint);
  }
  const envelope = {
    ok: false,
    error: sanitizeDeep(wireError),
    meta: buildMeta(opts.meta),
  };
  writeJson(process.stderr, envelope, pretty);
  return exitCodeOf(typed);
}

/**
 * Create a per-invocation output helper injected as ctx.out.
 * @param {{ pretty?: boolean }} flags
 */
function createOut(flags = {}) {
  return {
    ok(data, metaExtra) {
      ok(data, { pretty: flags.pretty, meta: metaExtra });
    },
    fail(err, metaExtra) {
      return fail(err, { pretty: flags.pretty, meta: metaExtra });
    },
  };
}

/**
 * Legacy adapter: wraps payload as success envelope.
 * Prefer ctx.out.ok in new code.
 * @param {unknown} data
 * @param {{ pretty?: boolean }} [flags]
 */
function printJson(data, flags = {}) {
  ok(data, { pretty: Boolean(flags && flags.pretty) });
}

/**
 * Legacy name for fail(); returns exit code.
 * @param {unknown} err
 * @returns {number}
 */
function printError(err) {
  return fail(err);
}

module.exports = {
  setActiveMeta,
  getActiveMeta,
  buildMeta,
  ok,
  fail,
  createOut,
  printJson,
  printError,
  UsageError,
  CONTRACT_VERSION,
};
