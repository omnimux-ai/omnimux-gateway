/**
 * omnimux tokens — API token management.
 *
 *   omnimux tokens                          list tokens (keys masked)
 *   omnimux tokens create <name> [--group=g] [--quota=usd]
 *   omnimux tokens switch-group <id> <group>
 *   omnimux tokens copy <id>                real key -> clipboard (never shown)
 *   omnimux tokens apply <id> <file>        real key -> __OMNIMUX_TOKEN_<id>__ placeholder
 *   omnimux tokens exec <id> [--timeout=sec] -- <cmd...>
 *                                     real key substituted into a child command
 *                                     (child timeout: 300s default)
 *
 * Endpoints (user-level only): /api/token/, /api/token/<id>, /api/token/<id>/key
 *
 * Security: the full key never reaches stdout/stderr. copy/apply/exec move it
 * straight from the API response into the clipboard / file / child process.
 */

const childProcess = require("child_process");
const http = require("../lib/http");
const { copyToClipboard } = require("../lib/clipboard");
const { injectPlaceholder, placeholderFor } = require("../lib/inject");
const { printJson, UsageError } = require("../lib/output");
const { sanitize } = require("../lib/mask");
const { QUOTA_PER_USD } = require("./balance");

function emitOk(ctx, data) {
  if (ctx.out) ctx.out.ok(data);
  else printJson(data, ctx.flags);
}

function parseTokenId(value) {
  if (!value || !/^\d+$/.test(String(value))) {
    throw new UsageError("token id must be a number");
  }
  return String(value);
}

function buildCreateBody(ctx) {
  const name = ctx.args[0];
  if (!name) {
    throw new UsageError("usage: omnimux tokens create <name> [--group=<group>] [--quota=<usd>]");
  }
  const body = { name };
  if (ctx.flags.group) body.group = String(ctx.flags.group);
  if (ctx.flags.quota !== undefined) {
    const dollars = Number(ctx.flags.quota);
    if (!Number.isFinite(dollars) || dollars < 0) {
      throw new UsageError("--quota must be a non-negative number (USD)");
    }
    body.remain_quota = Math.round(dollars * QUOTA_PER_USD);
    body.unlimited_quota = false;
  } else {
    body.unlimited_quota = true;
  }
  return body;
}

async function list(ctx) {
  const pageSize = ctx.flags.pageSize || "20";
  const data = await ctx.api("GET", `/api/token/?p=0&page_size=${encodeURIComponent(pageSize)}`);
  emitOk(ctx, data);
}

async function create(ctx) {
  const body = buildCreateBody(ctx);
  if (ctx.dryRun || ctx.flags.dryRun) {
    emitOk(ctx, {
      dry_run: true,
      would: "POST /api/token/",
      body,
    });
    return;
  }
  const data = await ctx.api("POST", "/api/token/", body);
  // Older server versions return no data.id on create; fall back to a masked
  // list call and pick the newest token with the same name. The list response
  // is already key-masked by the HTTP layer, so this adds no exposure.
  let tokenId = data && data.data && data.data.id != null ? data.data.id : null;
  if (tokenId == null) {
    tokenId = await resolveCreatedTokenId(ctx, body.name);
  }
  // The create response intentionally omits the real key. Never try to fetch
  // it here; direct the user to copy/apply/exec instead.
  const nextStep =
    tokenId != null
      ? `Use \`omnimux tokens copy ${tokenId}\`, \`omnimux tokens apply ${tokenId} <file>\`, or \`omnimux tokens exec ${tokenId} -- <cmd>\` to use the key securely. Do NOT fetch the key into chat or logs.`
      : "Could not resolve the new token's id from the server response. Run `omnimux tokens` to view the list, find the token's id, then use `omnimux tokens copy <id>`, `omnimux tokens apply <id> <file>`, or `omnimux tokens exec <id> -- <cmd>`. Do NOT fetch the key into chat or logs.";
  emitOk(ctx, { ...data, token_id: tokenId, next_step: nextStep });
}

/**
 * Best-effort id lookup after a create that returned no data.id (older
 * servers). Matches by name, picks the highest id (newest). Returns null when
 * the id cannot be determined; never throws.
 */
async function resolveCreatedTokenId(ctx, name) {
  try {
    const listing = await ctx.api("GET", "/api/token/?p=0&page_size=20");
    const payload = listing && listing.data ? listing.data : null;
    // Shape differs by server version: a plain array, or a paginated
    // { items: [...] } object.
    const items = Array.isArray(payload) ? payload : payload && Array.isArray(payload.items) ? payload.items : [];
    const matches = items.filter((t) => t && t.name === name && Number.isInteger(t.id));
    if (matches.length === 0) return null;
    matches.sort((a, b) => b.id - a.id);
    return matches[0].id;
  } catch {
    return null;
  }
}

async function switchGroup(ctx) {
  const [idArg, group] = ctx.args;
  const tokenId = parseTokenId(idArg);
  if (!group) {
    throw new UsageError("usage: omnimux tokens switch-group <id> <group>");
  }
  if (ctx.dryRun || ctx.flags.dryRun) {
    emitOk(ctx, {
      dry_run: true,
      would: "PUT /api/token/",
      token_id: Number(tokenId),
      group: String(group),
    });
    return;
  }
  const current = await ctx.api("GET", `/api/token/${tokenId}`);
  const existing = current && current.data ? current.data : {};
  const data = await ctx.api("PUT", "/api/token/", {
    ...existing,
    id: Number(tokenId),
    group: String(group),
  });
  emitOk(ctx, data);
}

async function copy(ctx) {
  const tokenId = parseTokenId(ctx.args[0]);
  const key = await http.fetchTokenKey(ctx.auth, tokenId);
  copyToClipboard(key);
  emitOk(ctx, { success: true, token_id: Number(tokenId), copied_to_clipboard: true });
}

async function apply(ctx) {
  const tokenId = parseTokenId(ctx.args[0]);
  const filePath = ctx.args[1];
  if (!filePath) {
    throw new UsageError(`usage: omnimux tokens apply <id> <file>  (file must contain ${placeholderFor("<id>")})`);
  }
  if (ctx.dryRun || ctx.flags.dryRun) {
    emitOk(ctx, {
      dry_run: true,
      would: "inject placeholder into file (no write performed)",
      token_id: Number(tokenId),
      file: filePath,
      placeholder: placeholderFor(tokenId),
    });
    return;
  }
  const key = await http.fetchTokenKey(ctx.auth, tokenId);
  injectPlaceholder(filePath, tokenId, key);
  emitOk(ctx, { success: true, token_id: Number(tokenId), file: filePath, applied: true });
}

const DEFAULT_EXEC_TIMEOUT_SECONDS = 300;

async function exec(ctx) {
  const tokenId = parseTokenId(ctx.args[0]);
  let timeoutSeconds = DEFAULT_EXEC_TIMEOUT_SECONDS;
  if (ctx.flags.timeout !== undefined) {
    timeoutSeconds = Number(ctx.flags.timeout);
    if (!Number.isFinite(timeoutSeconds) || timeoutSeconds <= 0) {
      throw new UsageError("--timeout must be a positive number (seconds)");
    }
  }
  const commandParts = ctx.args.slice(1);
  if (commandParts.length === 0) {
    throw new UsageError(`usage: omnimux tokens exec <id> -- <command with ${placeholderFor("<id>")} placeholder>`);
  }
  const placeholder = placeholderFor(tokenId);
  if (!commandParts.some((part) => String(part).includes(placeholder))) {
    throw new UsageError(`placeholder ${placeholder} not found in command`);
  }

  const key = await http.fetchTokenKey(ctx.auth, tokenId);
  // Substitute per argv element and spawn WITHOUT shell. Joining into a shell
  // string broke headers like -H "Authorization: Bearer sk-..." when callers
  // passed the header as one argv without nested quotes (→ HTTP 401).
  const argv = commandParts.map((part) => String(part).split(placeholder).join(key));
  const [cmd, ...args] = argv;
  if (!cmd) {
    throw new UsageError("command is empty after placeholder substitution");
  }

  const result = childProcess.spawnSync(cmd, args, {
    encoding: "utf-8",
    stdio: ["inherit", "pipe", "pipe"],
    timeout: Math.round(timeoutSeconds * 1000),
    maxBuffer: 20 * 1024 * 1024,
  });

  let exitCode = 0;
  let stdout = result.stdout || "";
  let stderr = result.stderr || "";
  if (result.error) {
    exitCode = 1;
    const msg = result.error.code === "ETIMEDOUT"
      ? `command timed out after ${timeoutSeconds}s\n`
      : `${result.error.message}\n`;
    stderr = (stderr || "") + msg;
  } else if (result.status !== null && result.status !== 0) {
    exitCode = result.status;
  } else if (result.signal) {
    exitCode = 1;
    stderr = (stderr || "") + `command killed by signal ${result.signal}\n`;
  }

  // Child output may echo the key; sanitize before emitting. This command
  // intentionally passes child output through verbatim (sanitized) rather
  // than the JSON contract, then propagates the child exit code.
  if (stdout) process.stdout.write(sanitize(stdout));
  if (stderr) process.stderr.write(sanitize(stderr));
  process.exit(exitCode);
}

const SUBCOMMANDS = { create, "switch-group": switchGroup, copy, apply, exec };

async function run(ctx) {
  const sub = ctx.args[0];
  if (!sub || sub === "list") {
    await list(ctx);
    return;
  }
  const handler = SUBCOMMANDS[sub];
  if (!handler) {
    throw new UsageError(
      `unknown tokens subcommand: ${sub} (expected: list, create, switch-group, copy, apply, exec)`
    );
  }
  await handler({ ...ctx, args: ctx.args.slice(1) });
}

module.exports = { run };
