#!/usr/bin/env node
/**
 * omnimux CLI entry point.
 *
 * Pure JavaScript (CommonJS), zero runtime dependencies.
 * Runs on Node >= 18 and Bun (native fetch).
 *
 * Agent RPC: success envelope on stdout, error envelope on stderr.
 * See cli/docs/ERROR_CONTRACT.md.
 */

const {
  requireBaseUrl,
  requireUserId,
  isBlank,
} = require("./lib/config");
const { resolveAccessToken } = require("./lib/secrets");
const { apiRequest } = require("./lib/http");
const {
  printError,
  createOut,
  setActiveMeta,
  ok,
} = require("./lib/output");
const { UsageError, AuthError } = require("./lib/errors");
const { sanitize } = require("./lib/mask");
const { maybeAutoUpdate } = require("./lib/update");
const { enforceRiskGate } = require("./lib/risk");
const { composeNotices } = require("./lib/notices");

const PKG = require("../package.json");

// --- Argument parsing ---

function parseArgs(argv) {
  const flags = {
    pretty: false,
    baseUrl: undefined,
    help: false,
    version: false,
    noOpen: false,
    noWait: false,
    check: false,
    force: false,
    noAutoUpdate: false,
    dryRun: false,
    yes: false,
    verify: false,
  };
  const args = [];
  let i = 0;
  let afterDashDash = false;
  while (i < argv.length) {
    const arg = argv[i];
    if (afterDashDash) {
      args.push(arg);
      i++;
      continue;
    }
    if (arg === "--") {
      afterDashDash = true;
      i++;
      continue;
    }
    if (arg === "--pretty") {
      flags.pretty = true;
    } else if (arg === "--no-open") {
      flags.noOpen = true;
    } else if (arg === "--no-wait") {
      flags.noWait = true;
    } else if (arg === "--check") {
      flags.check = true;
    } else if (arg === "--force") {
      flags.force = true;
    } else if (arg === "--no-auto-update") {
      flags.noAutoUpdate = true;
    } else if (arg === "--dry-run") {
      flags.dryRun = true;
    } else if (arg === "--yes") {
      flags.yes = true;
    } else if (arg === "--verify") {
      flags.verify = true;
    } else if (arg === "--user") {
      flags.user = true;
    } else if (arg === "--project") {
      flags.project = true;
    } else if (arg === "--mode") {
      flags.mode = argv[++i];
    } else if (arg.startsWith("--mode=")) {
      flags.mode = arg.slice("--mode=".length);
    } else if (arg === "--resume") {
      flags.resume = argv[++i];
    } else if (arg.startsWith("--resume=")) {
      flags.resume = arg.slice("--resume=".length);
    } else if (arg === "--help" || arg === "-h") {
      flags.help = true;
    } else if (arg === "--version" || arg === "-v") {
      flags.version = true;
    } else if (arg === "--base-url") {
      flags.baseUrl = argv[++i];
    } else if (arg.startsWith("--base-url=")) {
      flags.baseUrl = arg.slice("--base-url=".length);
    } else if (arg.startsWith("--group=")) {
      flags.group = arg.slice("--group=".length);
    } else if (arg === "--group") {
      flags.group = argv[++i];
    } else if (arg.startsWith("--quota=")) {
      flags.quota = arg.slice("--quota=".length);
    } else if (arg === "--quota") {
      flags.quota = argv[++i];
    } else if (arg.startsWith("--page-size=")) {
      flags.pageSize = arg.slice("--page-size=".length);
    } else if (arg.startsWith("--timeout=")) {
      flags.timeout = arg.slice("--timeout=".length);
    } else if (arg === "--timeout") {
      flags.timeout = argv[++i];
    } else if (arg.startsWith("--query=")) {
      flags.query = arg.slice("--query=".length);
    } else if (arg === "--query" || arg === "-q") {
      flags.query = argv[++i];
    } else if (arg.startsWith("--scene=")) {
      flags.scene = arg.slice("--scene=".length);
    } else if (arg === "--scene") {
      flags.scene = argv[++i];
    } else if (arg.startsWith("--category=")) {
      flags.category = arg.slice("--category=".length);
    } else if (arg === "--category") {
      flags.category = argv[++i];
    } else if (arg.startsWith("--language=")) {
      flags.language = arg.slice("--language=".length);
    } else if (arg === "--language" || arg === "--lang") {
      flags.language = argv[++i];
    } else if (arg.startsWith("--lang=")) {
      flags.language = arg.slice("--lang=".length);
    } else if (arg.startsWith("--model=")) {
      flags.model = arg.slice("--model=".length);
    } else if (arg === "--model" || arg === "-m") {
      flags.model = argv[++i];
    } else if (arg.startsWith("--limit=")) {
      flags.limit = arg.slice("--limit=".length);
    } else if (arg === "--limit") {
      flags.limit = argv[++i];
    } else if (arg.startsWith("--")) {
      // Pass through unknown flags for subcommands (e.g. social connect --platform tiktok).
      args.push(arg);
      if (!arg.includes("=") && i + 1 < argv.length && !String(argv[i + 1]).startsWith("-")) {
        args.push(argv[++i]);
      }
    } else {
      args.push(arg);
    }
    i++;
  }
  return { flags, args };
}

// auth: "none"  — no base_url or token needed
// auth: "base"  — base_url needed, token optional
// auth: "full"  — base_url + access token + user id required
// Static requires so `bun build --compile` embeds every command module.
const COMMANDS = {
  models: { run: require("./commands/models").run, auth: "full" },
  pricing: { run: require("./commands/pricing").run, auth: "base" },
  groups: { run: require("./commands/groups").run, auth: "full" },
  balance: { run: require("./commands/balance").run, auth: "full" },
  tokens: { run: require("./commands/tokens").run, auth: "full" },
  voices: { run: require("./commands/voices").run, auth: "none" },
  login: { run: require("./commands/login").run, auth: "base" },
  logout: { run: require("./commands/logout").run, auth: "none" },
  config: { run: require("./commands/config").run, auth: "none" },
  "scan-config": { run: require("./commands/scan-config").run, auth: "none" },
  update: { run: require("./commands/update").run, auth: "none" },
  upgrade: { run: require("./commands/update").run, auth: "none" },
  skill: { run: require("./commands/skill").run, auth: "none" },
  social: { run: require("./commands/social").run, auth: "full" },
  hosted: { run: require("./commands/hosted").run, auth: "full" },
  influencer: { run: require("./commands/influencer").run, auth: "full" },
  inspiration: { run: require("./commands/inspiration").run, auth: "full" },
  schema: { run: require("./commands/schema").run, auth: "none" },
  help: { run: require("./commands/help").run, auth: "none" },
};

function buildAuth(flags, level) {
  if (level === "none") return { baseUrl: "", accessToken: "", userId: "" };
  const { value: baseUrl } = requireBaseUrl(flags.baseUrl);
  if (level === "base") return { baseUrl, accessToken: "", userId: "" };
  const { value: accessToken } = resolveAccessToken();
  if (isBlank(accessToken)) {
    throw new AuthError("access token is not configured", {
      subtype: "token_missing",
      hint: "sign in or set a token, then retry",
      recovery: { command: "omnimux", args: ["login", "--no-wait"] },
    });
  }
  const { value: userId } = requireUserId();
  return { baseUrl, accessToken, userId };
}

/**
 * Resolve leaf name for meta.command (best-effort).
 */
function resolveCommandMetaName(commandName, args) {
  if (commandName === "influencer") {
    if (args[0] === "options") return "influencer.options";
    return [commandName, args[0], args[1]].filter(Boolean).join(".");
  }
  if (commandName === "login" && args[0] === "status") return "login.status";
  if (commandName === "tokens" && args[0]) {
    const sub = args[0];
    if (["create", "switch-group", "copy", "apply", "exec"].includes(sub)) {
      return `tokens.${sub}`;
    }
  }
  if (commandName === "skill" && args[0] === "read") return "skill.read";
  if (commandName === "hosted" && args[0]) {
    const sub = args[0];
    if (
      [
        "status",
        "skus",
        "channels",
        "channel",
        "brand",
        "post",
        "batch",
        "tasks",
        "task",
        "cancel",
        "upload",
        "buy",
        "addon",
        "renew",
        "order",
      ].includes(sub)
    ) {
      return `hosted.${sub}`;
    }
  }
  return commandName;
}

async function main() {
  const { flags, args } = parseArgs(process.argv.slice(2));

  if (flags.version) {
    setActiveMeta({ command: "version", cli_version: PKG.version });
    ok({ version: PKG.version }, { pretty: flags.pretty });
    return;
  }

  let commandName = args[0];
  let cmdArgs = args.slice(1);

  if (!commandName || flags.help || commandName === "help") {
    commandName = "help";
    cmdArgs = args[0] === "help" ? args.slice(1) : [];
  }

  const command = COMMANDS[commandName];
  if (!command) {
    setActiveMeta({ command: commandName || "unknown", cli_version: PKG.version });
    throw new UsageError(`unknown command: ${commandName} (run \`omnimux help\`)`, {
      subtype: "unknown_command",
      recovery: { command: "omnimux", args: ["help"] },
    });
  }

  const metaCommand = resolveCommandMetaName(commandName, cmdArgs);
  setActiveMeta({ command: metaCommand, cli_version: PKG.version });

  // Risk gate before auto-update / secrets / network (fail closed).
  const risk = enforceRiskGate(metaCommand, flags, process.argv.slice(2));

  // login status does not need base auth level "base" device flow — override
  let authLevel = command.auth;
  if (commandName === "login" && cmdArgs[0] === "status") {
    authLevel = "none";
  }
  // Dry-run previews that need no API: skip credential requirements.
  if (risk.dryRun && risk.leaf && risk.leaf.dryRun) {
    authLevel = "none";
  }

  // Background self-update is a side effect — never under --dry-run.
  let autoUpdateResult = null;
  if (!flags.dryRun) {
    autoUpdateResult = await maybeAutoUpdate({
      currentVersion: PKG.version,
      flags,
      commandName,
      onUpdated: (result) => {
        void result;
      },
    });
  }

  // Agent recovery notices on success envelopes (suppressible via env).
  const notices = composeNotices({
    cliVersion: PKG.version,
    autoUpdateResult,
    cwd: process.cwd(),
  });
  setActiveMeta({
    command: metaCommand,
    cli_version: PKG.version,
    notices,
  });

  const auth = buildAuth(flags, authLevel);
  const out = createOut(flags);
  const ctx = {
    flags,
    args: cmdArgs,
    auth,
    out,
    dryRun: risk.dryRun,
    api: (method, urlPath, body, extraHeaders, requestOptions) =>
      apiRequest(auth, method, urlPath, body, extraHeaders, requestOptions),
  };
  await command.run(ctx);
}

if (require.main === module) {
  main().catch((err) => {
    process.exitCode = printError(err instanceof Error ? err : new Error(sanitize(String(err))));
  });
}

// Exported for unit tests (not a public CLI API).
module.exports = { parseArgs, COMMANDS, buildAuth };
