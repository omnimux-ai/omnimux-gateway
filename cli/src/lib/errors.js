/**
 * Typed CLI errors for Agent-readable envelopes.
 *
 * Wire fields are allowlisted in lib/output.js — never put secrets in message/hint.
 * See cli/docs/ERROR_CONTRACT.md.
 */

const CONTRACT_VERSION = 1;

/** @typedef {"usage"|"config"|"auth"|"api"|"network"|"internal"|"confirmation"} ErrorType */

const EXIT_BY_TYPE = {
  usage: 3,
  config: 2,
  auth: 2,
  api: 1,
  network: 4,
  internal: 5,
  confirmation: 10,
};

/**
 * @param {ErrorType} type
 * @returns {number}
 */
function exitCodeForType(type) {
  return EXIT_BY_TYPE[type] != null ? EXIT_BY_TYPE[type] : 5;
}

/**
 * @param {unknown} err
 * @returns {number}
 */
function exitCodeOf(err) {
  if (err && typeof err.exitCode === "number") return err.exitCode;
  if (err && typeof err.type === "string" && EXIT_BY_TYPE[err.type] != null) {
    return EXIT_BY_TYPE[err.type];
  }
  return 1;
}

/**
 * Base typed error. Subclasses pin `type` and default exit codes.
 */
class CliError extends Error {
  /**
   * @param {object} opts
   * @param {ErrorType} opts.type
   * @param {string} opts.subtype
   * @param {string} opts.message
   * @param {string} [opts.hint]
   * @param {string|null} [opts.param]
   * @param {boolean} [opts.retryable]
   * @param {{ command: string, args: string[] }} [opts.recovery]
   * @param {string} [opts.requestId]
   * @param {string} [opts.operationId] client creation identity
   * @param {string} [opts.taskId] original task identity
   * @param {string} [opts.action]
   * @param {string} [opts.risk]
   * @param {number} [opts.status] HTTP status when known
   * @param {Error} [opts.cause]
   */
  constructor(opts) {
    super(opts.message);
    this.name = "CliError";
    this.type = opts.type;
    this.subtype = opts.subtype;
    this.hint = opts.hint || undefined;
    this.param = opts.param != null ? opts.param : undefined;
    this.retryable = Boolean(opts.retryable);
    this.recovery = opts.recovery || undefined;
    this.requestId = opts.requestId || undefined;
    this.operationId = opts.operationId || undefined;
    this.taskId = opts.taskId || undefined;
    this.action = opts.action || undefined;
    this.risk = opts.risk || undefined;
    this.status = opts.status || 0;
    this.exitCode = exitCodeForType(opts.type);
    if (opts.cause) this.cause = opts.cause;
  }

  withHint(hint) {
    this.hint = hint;
    return this;
  }

  withParam(param) {
    this.param = param;
    return this;
  }

  withRecovery(command, args) {
    this.recovery = { command, args: Array.isArray(args) ? args : [] };
    return this;
  }

  withRetryable(v = true) {
    this.retryable = Boolean(v);
    return this;
  }
}

class UsageError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, param?: string, hint?: string, recovery?: { command: string, args: string[] } }} [opts]
   */
  constructor(message, opts = {}) {
    super({
      type: "usage",
      subtype: opts.subtype || "invalid_argument",
      message,
      hint: opts.hint,
      param: opts.param,
      recovery: opts.recovery,
      retryable: false,
    });
    this.name = "UsageError";
  }
}

class ConfigError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, hint?: string, recovery?: { command: string, args: string[] } }} [opts]
   */
  constructor(message, opts = {}) {
    super({
      type: "config",
      subtype: opts.subtype || "not_configured",
      message,
      hint: opts.hint,
      recovery: opts.recovery,
      retryable: false,
    });
    this.name = "ConfigError";
  }
}

class AuthError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, hint?: string, recovery?: { command: string, args: string[] } }} [opts]
   */
  constructor(message, opts = {}) {
    super({
      type: "auth",
      subtype: opts.subtype || "token_missing",
      message,
      requestId: opts.requestId,
      hint: opts.hint,
      recovery: opts.recovery,
      retryable: false,
    });
    this.name = "AuthError";
  }
}

class ApiError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, status?: number, hint?: string, retryable?: boolean, requestId?: string, recovery?: { command: string, args: string[] } }} [opts]
   */
  constructor(message, opts = {}) {
    // Back-compat: `new ApiError(msg, statusNumber)` used by older call sites.
    if (typeof opts === "number") {
      opts = { status: opts, subtype: "upstream" };
    }
    super({
      type: "api",
      subtype: opts.subtype || "upstream",
      message,
      status: opts.status || 0,
      hint: opts.hint,
      retryable: Boolean(opts.retryable),
      requestId: opts.requestId,
      operationId: opts.operationId,
      taskId: opts.taskId,
      recovery: opts.recovery,
    });
    this.name = "ApiError";
  }
}

class NetworkError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, hint?: string, cause?: Error }} [opts]
   */
  constructor(message, opts = {}) {
    super({
      type: "network",
      subtype: opts.subtype || "transport",
      message,
      hint: opts.hint,
      retryable: true,
      cause: opts.cause,
    });
    this.name = "NetworkError";
  }
}

class InternalError extends CliError {
  /**
   * @param {string} message
   * @param {{ subtype?: string, cause?: Error, hint?: string }} [opts]
   */
  constructor(message, opts = {}) {
    super({
      type: "internal",
      subtype: opts.subtype || "unknown",
      message,
      hint: opts.hint,
      retryable: false,
      cause: opts.cause,
    });
    this.name = "InternalError";
  }
}

class ConfirmationRequiredError extends CliError {
  /**
   * @param {string} message
   * @param {{ action: string, risk?: string, hint?: string, recovery?: { command: string, args: string[] } }} opts
   */
  constructor(message, opts) {
    super({
      type: "confirmation",
      subtype: "confirmation_required",
      message,
      action: opts.action,
      risk: opts.risk || "high-risk-write",
      hint: opts.hint || "add --yes to confirm after explicit user approval",
      recovery: opts.recovery,
      retryable: false,
    });
    this.name = "ConfirmationRequiredError";
  }
}

/**
 * Normalize unknown throws into a typed CliError for the envelope path.
 * @param {unknown} err
 * @returns {CliError}
 */
function asCliError(err) {
  if (err instanceof CliError) return err;
  // Legacy errors that only set exitCode
  if (err && typeof err === "object" && typeof err.exitCode === "number" && err.message) {
    const exit = err.exitCode;
    const message = String(err.message);
    if (exit === 3) return new UsageError(message);
    if (exit === 2) {
      // Distinguishing config vs auth is imperfect; prefer auth keywords.
      if (/token|login|auth/i.test(message)) {
        return new AuthError(message, { subtype: "token_missing" });
      }
      return new ConfigError(message);
    }
    if (exit === 4) return new NetworkError(message);
    if (exit === 5) return new InternalError(message);
    if (exit === 10) {
      return new ConfirmationRequiredError(message, { action: "unknown" });
    }
    return new ApiError(message, { subtype: "upstream" });
  }
  if (err instanceof Error) {
    return new InternalError(err.message || "unknown error", {
      subtype: "unknown",
      cause: err,
    });
  }
  return new InternalError(String(err || "unknown error"));
}

/**
 * Build the allowlisted error object for JSON serialization.
 * @param {CliError} err
 */
function toWireError(err) {
  const e = asCliError(err);
  /** @type {Record<string, unknown>} */
  const wire = {
    type: e.type,
    subtype: e.subtype,
    message: e.message,
    retryable: Boolean(e.retryable),
  };
  if (e.hint) wire.hint = e.hint;
  if (e.param != null && e.param !== "") wire.param = e.param;
  if (e.recovery && e.recovery.command) {
    wire.recovery = {
      command: e.recovery.command,
      args: Array.isArray(e.recovery.args) ? e.recovery.args : [],
    };
  }
  if (e.requestId) wire.request_id = e.requestId;
  if (e.operationId) wire.operation_id = e.operationId;
  if (e.taskId) wire.task_id = e.taskId;
  if (e.action) wire.action = e.action;
  if (e.risk) wire.risk = e.risk;
  if (e.status) wire.status = e.status;
  return wire;
}

module.exports = {
  CONTRACT_VERSION,
  EXIT_BY_TYPE,
  exitCodeForType,
  exitCodeOf,
  CliError,
  UsageError,
  ConfigError,
  AuthError,
  ApiError,
  NetworkError,
  InternalError,
  ConfirmationRequiredError,
  asCliError,
  toWireError,
};
