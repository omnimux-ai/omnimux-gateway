/**
 * Self-update helpers for the omnimux CLI.
 *
 * Channels:
 *   - npm    — global @omnimux/cli install (node_modules path)
 *   - binary — prebuilt install.sh / GitHub Release binary
 *   - source — local checkout / bun src/bin.js (no auto-update)
 *   - unknown
 *
 * Also migrates legacy geminix.cc base_url in config.json to the official
 * default (omnimux.ai) so domain cutover does not require a separate step.
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");
const { spawnSync } = require("child_process");
const {
  OMNIMUX_DEFAULT_BASE_URL,
  configDir,
  loadConfig,
  saveConfig,
  normalizeBaseUrl,
  isBlank,
  resolveAutoUpdate,
} = require("./config");

const NPM_PACKAGE = "@omnimux/cli";
const RELEASE_REPO = "laozhong86/omnimux-cli";
const LEGACY_BASE_HOSTS = new Set(["geminix.cc", "www.geminix.cc", "api.geminix.cc"]);
/** How often background auto-update may hit the network (default 24h). */
const DEFAULT_AUTO_UPDATE_INTERVAL_MS = 24 * 60 * 60 * 1000;
/** Short timeout for background version checks so offline users are not stalled. */
const AUTO_UPDATE_FETCH_TIMEOUT_MS = 5000;

function abortSignalAfter(ms) {
  if (typeof AbortSignal !== "undefined" && typeof AbortSignal.timeout === "function") {
    return AbortSignal.timeout(ms);
  }
  const controller = new AbortController();
  setTimeout(() => controller.abort(), ms).unref?.();
  return controller.signal;
}

function updateStatePath() {
  return path.join(configDir(), "update-state.json");
}

function loadUpdateState() {
  const p = updateStatePath();
  if (!fs.existsSync(p)) return {};
  try {
    const raw = JSON.parse(fs.readFileSync(p, "utf8"));
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
    return {
      last_check_at:
        typeof raw.last_check_at === "number" && Number.isFinite(raw.last_check_at)
          ? raw.last_check_at
          : undefined,
      last_result: raw.last_result ? String(raw.last_result) : undefined,
      last_version: raw.last_version ? String(raw.last_version) : undefined,
    };
  } catch {
    return {};
  }
}

function saveUpdateState(state) {
  const dir = configDir();
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  }
  const clean = {
    last_check_at: state.last_check_at,
    last_result: state.last_result || null,
    last_version: state.last_version || null,
  };
  fs.writeFileSync(updateStatePath(), JSON.stringify(clean, null, 2) + "\n", {
    mode: 0o600,
  });
  return updateStatePath();
}

/**
 * @param {number} [now]
 * @param {number} [intervalMs]
 * @param {{ last_check_at?: number }} [state]
 */
function shouldRunAutoUpdateCheck(
  now = Date.now(),
  intervalMs = DEFAULT_AUTO_UPDATE_INTERVAL_MS,
  state = loadUpdateState()
) {
  if (!state || !state.last_check_at) return true;
  return now - state.last_check_at >= intervalMs;
}

/**
 * @param {string} version
 * @returns {[number, number, number] | null}
 */
function parseSemver(version) {
  const s = String(version || "")
    .trim()
    .replace(/^cli-v/i, "")
    .replace(/^v/i, "");
  const m = s.match(/^(\d+)\.(\d+)\.(\d+)/);
  if (!m) return null;
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

/**
 * @param {string} a
 * @param {string} b
 * @returns {number} -1 if a<b, 0 equal/unparseable, 1 if a>b
 */
function compareSemver(a, b) {
  const pa = parseSemver(a);
  const pb = parseSemver(b);
  if (!pa || !pb) return 0;
  for (let i = 0; i < 3; i++) {
    if (pa[i] > pb[i]) return 1;
    if (pa[i] < pb[i]) return -1;
  }
  return 0;
}

function stripVersionPrefix(version) {
  return String(version || "")
    .trim()
    .replace(/^cli-v/i, "")
    .replace(/^v/i, "");
}

/**
 * Detect how this CLI process was installed.
 * @param {{ argv?: string[], execPath?: string }} [proc]
 */
function detectInstallChannel(proc = process) {
  const argv = Array.isArray(proc.argv) ? proc.argv : [];
  let argv1 = argv[1] ? String(argv[1]) : "";
  // nvm/npm put a symlink at .../bin/omnimux → realpath to node_modules/@omnimux/cli/...
  if (argv1) {
    try {
      if (fs.existsSync(argv1)) {
        argv1 = fs.realpathSync(argv1);
      }
    } catch {
      // keep original
    }
  }
  const execPath = proc.execPath ? String(proc.execPath) : "";
  const execBase = path.basename(execPath).toLowerCase();
  const argv1Norm = argv1.replace(/\\/g, "/");

  if (
    argv1Norm.includes("/@omnimux/cli/") ||
    argv1Norm.includes("/node_modules/@omnimux/cli/")
  ) {
    return { channel: "npm", installPath: argv1 };
  }

  if (argv1Norm.endsWith("/src/bin.js") || argv1Norm.endsWith("/cli/src/bin.js")) {
    if (argv1Norm.includes("/node_modules/")) {
      return { channel: "npm", installPath: argv1 };
    }
    return { channel: "source", installPath: argv1 };
  }

  // npm bin shim path still named .../bin/omnimux after failed realpath
  if (
    argv1Norm.endsWith("/bin/omnimux") ||
    argv1Norm.endsWith("/bin/omnimux.js") ||
    argv1Norm.endsWith("\\bin\\omnimux") ||
    argv1Norm.endsWith("\\bin\\omnimux.cmd")
  ) {
    const maybePkg = path.resolve(path.dirname(argv1), "..", "lib", "node_modules", "@omnimux", "cli");
    if (fs.existsSync(path.join(maybePkg, "package.json"))) {
      return { channel: "npm", installPath: path.join(maybePkg, "src", "bin.js") };
    }
  }

  const isInterpreter = /^(node|nodejs|bun)(\.exe)?$/i.test(execBase);
  if (!isInterpreter && execPath) {
    return { channel: "binary", installPath: execPath };
  }

  if (argv1) {
    return { channel: "source", installPath: argv1 };
  }
  return { channel: "unknown", installPath: execPath || "" };
}

/**
 * Rewrite config.json base_url from legacy geminix hosts to the official URL.
 * Does not touch OMNIMUX_BASE_URL env (user-owned).
 * @returns {{ from: string, to: string } | null}
 */
function migrateLegacyBaseUrl() {
  const cfg = loadConfig();
  if (isBlank(cfg.base_url)) return null;
  let host = "";
  try {
    host = new URL(cfg.base_url).hostname.toLowerCase();
  } catch {
    return null;
  }
  if (!LEGACY_BASE_HOSTS.has(host)) return null;
  const from = normalizeBaseUrl(cfg.base_url);
  const to = normalizeBaseUrl(OMNIMUX_DEFAULT_BASE_URL);
  if (from === to) return null;
  cfg.base_url = to;
  saveConfig(cfg);
  return { from, to };
}

/**
 * @param {typeof fetch} [fetchImpl]
 * @param {number} [timeoutMs]
 */
async function fetchLatestNpmVersion(fetchImpl = fetch, timeoutMs = 20000) {
  const url = `https://registry.npmjs.org/${NPM_PACKAGE}/latest`;
  const res = await fetchImpl(url, {
    headers: { Accept: "application/json" },
    signal: abortSignalAfter(timeoutMs),
  });
  if (!res.ok) {
    throw new Error(`npm registry returned HTTP ${res.status} for ${NPM_PACKAGE}`);
  }
  const json = await res.json();
  const version = stripVersionPrefix(json && json.version);
  if (!version) throw new Error("npm registry response missing version");
  return { version, source: "npm", tag: version };
}

/**
 * @param {typeof fetch} [fetchImpl]
 * @param {number} [timeoutMs]
 */
async function fetchLatestGithubVersion(fetchImpl = fetch, timeoutMs = 20000) {
  const url = `https://api.github.com/repos/${RELEASE_REPO}/releases/latest`;
  const res = await fetchImpl(url, {
    headers: {
      Accept: "application/vnd.github+json",
      "User-Agent": "omnimux-cli",
    },
    signal: abortSignalAfter(timeoutMs),
  });
  if (!res.ok) {
    throw new Error(`GitHub releases returned HTTP ${res.status} for ${RELEASE_REPO}`);
  }
  const json = await res.json();
  const tag = String((json && json.tag_name) || "");
  const version = stripVersionPrefix(tag);
  if (!version) throw new Error("GitHub release missing tag_name");
  return { version, source: "github", tag: tag || `cli-v${version}` };
}

/**
 * @param {"npm"|"binary"|"source"|"unknown"} channel
 * @param {typeof fetch} [fetchImpl]
 * @param {number} [timeoutMs]
 */
async function fetchLatestVersion(channel, fetchImpl = fetch, timeoutMs = 20000) {
  if (channel === "binary") {
    return fetchLatestGithubVersion(fetchImpl, timeoutMs);
  }
  // npm / source / unknown: prefer npm registry (matches published package)
  try {
    return await fetchLatestNpmVersion(fetchImpl, timeoutMs);
  } catch (npmErr) {
    if (channel === "npm") throw npmErr;
    return fetchLatestGithubVersion(fetchImpl, timeoutMs);
  }
}

function resolvePlatformAsset() {
  const osName = os.platform(); // darwin | linux | win32
  const archRaw = os.arch(); // arm64 | x64 | ...
  let platform;
  if (osName === "darwin") platform = "darwin";
  else if (osName === "linux") platform = "linux";
  else if (osName === "win32") platform = "windows";
  else {
    return { error: `unsupported OS for binary update: ${osName}` };
  }

  let arch;
  if (archRaw === "arm64") arch = "arm64";
  else if (archRaw === "x64") arch = "x64";
  else {
    return { error: `unsupported architecture for binary update: ${archRaw}` };
  }

  if (platform === "linux" && arch === "arm64") {
    return {
      error:
        "no prebuilt linux/arm64 binary; install via npm: npm i -g @omnimux/cli",
    };
  }
  if (platform === "windows" && arch === "arm64") {
    return {
      error:
        "no prebuilt windows/arm64 binary; install via npm: npm i -g @omnimux/cli",
    };
  }

  const asset =
    platform === "windows"
      ? `omnimux-windows-x64.exe`
      : `omnimux-${platform}-${arch}`;
  return { platform, arch, asset };
}

/**
 * @param {string} command
 * @param {string[]} args
 * @param {{ spawnSyncFn?: typeof spawnSync }} [opts]
 */
function runCommand(command, args, opts = {}) {
  const spawnSyncFn = opts.spawnSyncFn || spawnSync;
  const result = spawnSyncFn(command, args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  return {
    status: result.status,
    error: result.error ? String(result.error.message || result.error) : null,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
  };
}

/**
 * @param {string} targetVersion
 * @param {{ spawnSyncFn?: typeof spawnSync }} [opts]
 */
function updateViaNpm(targetVersion, opts = {}) {
  const version = stripVersionPrefix(targetVersion) || "latest";
  const spec = `${NPM_PACKAGE}@${version}`;

  // Prefer npm; fall back to bun for users who only have bun globally.
  const attempts = [
    ["npm", ["install", "-g", spec]],
    ["bun", ["add", "-g", spec]],
  ];
  const tried = [];
  for (const [cmd, args] of attempts) {
    const result = runCommand(cmd, args, opts);
    tried.push({ command: [cmd, ...args].join(" "), status: result.status, error: result.error });
    if (!result.error && result.status === 0) {
      return {
        ok: true,
        method: cmd,
        command: [cmd, ...args].join(" "),
        stdout: result.stdout,
        stderr: result.stderr,
      };
    }
  }
  return {
    ok: false,
    method: null,
    command: tried.map((t) => t.command).join(" || "),
    error:
      "failed to run npm/bun global install. Install manually: npm i -g " +
      spec,
    tried,
  };
}

/**
 * Download a release asset and replace the current binary.
 * @param {string} targetVersion
 * @param {string} installPath
 * @param {{ fetchImpl?: typeof fetch, writeFileSyncFn?: Function, renameSyncFn?: Function, chmodSyncFn?: Function, createHashFn?: Function }} [opts]
 */
async function updateViaBinary(targetVersion, installPath, opts = {}) {
  const fetchImpl = opts.fetchImpl || fetch;
  const writeFileSyncFn = opts.writeFileSyncFn || fs.writeFileSync.bind(fs);
  const renameSyncFn = opts.renameSyncFn || fs.renameSync.bind(fs);
  const chmodSyncFn = opts.chmodSyncFn || fs.chmodSync.bind(fs);
  const createHashFn = opts.createHashFn || crypto.createHash.bind(crypto);
  const copyFileSyncFn = opts.copyFileSyncFn || fs.copyFileSync.bind(fs);
  const unlinkSyncFn = opts.unlinkSyncFn || fs.unlinkSync.bind(fs);
  const mkdtempSyncFn = opts.mkdtempSyncFn || fs.mkdtempSync.bind(fs);

  if (isBlank(installPath)) {
    return { ok: false, error: "cannot determine binary install path" };
  }

  const assetInfo = resolvePlatformAsset();
  if (assetInfo.error) {
    return { ok: false, error: assetInfo.error };
  }

  const version = stripVersionPrefix(targetVersion);
  const tag = `cli-v${version}`;
  const base =
    version && version !== "latest"
      ? `https://github.com/${RELEASE_REPO}/releases/download/${tag}`
      : `https://github.com/${RELEASE_REPO}/releases/latest/download`;
  const url = `${base}/${assetInfo.asset}`;
  const sumsUrl = `${base}/SHA256SUMS`;

  const res = await fetchImpl(url, {
    headers: { "User-Agent": "omnimux-cli" },
    signal: abortSignalAfter(120000),
    redirect: "follow",
  });
  if (!res.ok) {
    return {
      ok: false,
      error: `download failed HTTP ${res.status} for ${assetInfo.asset}`,
      url,
    };
  }
  const buf = Buffer.from(await res.arrayBuffer());

  // Optional checksum
  let checksumOk = null;
  try {
    const sumsRes = await fetchImpl(sumsUrl, {
      headers: { "User-Agent": "omnimux-cli" },
      signal: abortSignalAfter(20000),
      redirect: "follow",
    });
    if (sumsRes.ok) {
      const text = await sumsRes.text();
      const line = text
        .split("\n")
        .map((l) => l.trim())
        .find((l) => l.endsWith(assetInfo.asset));
      if (line) {
        const expected = line.split(/\s+/)[0];
        const actual = createHashFn("sha256").update(buf).digest("hex");
        if (expected && actual !== expected) {
          return {
            ok: false,
            error: `SHA256 mismatch for ${assetInfo.asset}`,
            expected,
            actual,
            url,
          };
        }
        checksumOk = true;
      }
    }
  } catch {
    checksumOk = null;
  }

  const tmpDir = mkdtempSyncFn(path.join(os.tmpdir(), "omnimux-update-"));
  const tmpFile = path.join(tmpDir, assetInfo.asset);
  writeFileSyncFn(tmpFile, buf, { mode: 0o755 });
  try {
    chmodSyncFn(tmpFile, 0o755);
  } catch {
    // Windows may ignore mode
  }

  // Replace in place: write to sibling temp then rename. On Windows, renaming
  // over a running executable can fail — fall back to .new + instruction.
  const dest = installPath;
  const sibling = `${dest}.new`;
  try {
    copyFileSyncFn(tmpFile, sibling);
    try {
      chmodSyncFn(sibling, 0o755);
    } catch {
      /* ignore */
    }
    try {
      renameSyncFn(sibling, dest);
    } catch {
      // Leave .new for the user / next launch; try direct overwrite of dest.
      try {
        writeFileSyncFn(dest, buf, { mode: 0o755 });
        try {
          unlinkSyncFn(sibling);
        } catch {
          /* ignore */
        }
      } catch (err) {
        return {
          ok: false,
          error:
            `cannot replace binary at ${dest}: ${err && err.message ? err.message : String(err)}. ` +
            `Re-run with write permission, or: curl -fsSL https://omnimux.ai/install.sh | bash`,
          url,
          partial_path: sibling,
        };
      }
    }
  } finally {
    try {
      unlinkSyncFn(tmpFile);
    } catch {
      /* ignore */
    }
    try {
      fs.rmdirSync(tmpDir);
    } catch {
      /* ignore */
    }
  }

  return {
    ok: true,
    method: "binary",
    command: `download ${url} -> ${dest}`,
    url,
    asset: assetInfo.asset,
    checksum_ok: checksumOk,
    install_path: dest,
  };
}

/**
 * Manual install hints when auto-update is not possible.
 * @param {"npm"|"binary"|"source"|"unknown"} channel
 * @param {string} [version]
 */
function manualInstallHints(channel, version) {
  const v = stripVersionPrefix(version) || "latest";
  const npm = `npm install -g ${NPM_PACKAGE}@${v === "latest" ? "latest" : v}`;
  const binary = "curl -fsSL https://omnimux.ai/install.sh | bash";
  if (channel === "npm") {
    return { primary: npm, alternative: binary };
  }
  if (channel === "binary") {
    return { primary: binary, alternative: npm };
  }
  return {
    primary: npm,
    alternative: binary,
    note:
      channel === "source"
        ? "Running from source; pull latest monorepo cli/ or install a release build."
        : "Could not detect install channel; use npm or the install script.",
  };
}

/**
 * Orchestrate check / update.
 *
 * @param {{
 *   currentVersion: string,
 *   checkOnly?: boolean,
 *   force?: boolean,
 *   proc?: NodeJS.Process,
 *   fetchImpl?: typeof fetch,
 *   spawnSyncFn?: typeof spawnSync,
 *   skipMigrate?: boolean,
 *   fetchTimeoutMs?: number,
 * }} options
 */
async function performUpdate(options) {
  const currentVersion = stripVersionPrefix(options.currentVersion);
  const checkOnly = !!options.checkOnly;
  const force = !!options.force;
  const channelInfo = detectInstallChannel(options.proc || process);
  const channel = channelInfo.channel;
  const fetchTimeoutMs =
    typeof options.fetchTimeoutMs === "number" && options.fetchTimeoutMs > 0
      ? options.fetchTimeoutMs
      : 20000;

  let latest;
  try {
    latest = await fetchLatestVersion(channel, options.fetchImpl || fetch, fetchTimeoutMs);
  } catch (err) {
    return {
      success: false,
      action: "error",
      channel,
      install_path: channelInfo.installPath || null,
      current_version: currentVersion,
      latest_version: null,
      error: err && err.message ? err.message : String(err),
      manual: manualInstallHints(channel),
      base_url_migrated: null,
    };
  }

  const cmp = compareSemver(currentVersion, latest.version);
  const needsUpdate = force || cmp < 0;
  const alreadyLatest = !force && cmp === 0;
  const newerLocal = !force && cmp > 0;

  let baseUrlMigrated = null;
  if (!options.skipMigrate) {
    try {
      baseUrlMigrated = migrateLegacyBaseUrl();
    } catch {
      baseUrlMigrated = null;
    }
  }

  if (checkOnly) {
    return {
      success: true,
      action: "check_only",
      channel,
      install_path: channelInfo.installPath || null,
      current_version: currentVersion,
      latest_version: latest.version,
      latest_source: latest.source,
      update_available: needsUpdate && !newerLocal,
      already_latest: alreadyLatest,
      base_url_migrated: baseUrlMigrated,
      manual: needsUpdate ? manualInstallHints(channel, latest.version) : null,
    };
  }

  if (alreadyLatest || newerLocal) {
    return {
      success: true,
      action: "already_latest",
      channel,
      install_path: channelInfo.installPath || null,
      current_version: currentVersion,
      latest_version: latest.version,
      latest_source: latest.source,
      update_available: false,
      already_latest: alreadyLatest,
      base_url_migrated: baseUrlMigrated,
      message: newerLocal
        ? `local version ${currentVersion} is newer than registry ${latest.version}`
        : `already on latest version ${currentVersion}`,
    };
  }

  if (channel === "source" || channel === "unknown") {
    return {
      success: false,
      action: "manual_required",
      channel,
      install_path: channelInfo.installPath || null,
      current_version: currentVersion,
      latest_version: latest.version,
      latest_source: latest.source,
      update_available: true,
      base_url_migrated: baseUrlMigrated,
      manual: manualInstallHints(channel, latest.version),
      error:
        channel === "source"
          ? "running from source; install a release build to enable self-update"
          : "could not detect install channel; use manual install",
    };
  }

  let installResult;
  if (channel === "npm") {
    installResult = updateViaNpm(latest.version, { spawnSyncFn: options.spawnSyncFn });
  } else {
    installResult = await updateViaBinary(latest.version, channelInfo.installPath, {
      fetchImpl: options.fetchImpl,
      writeFileSyncFn: options.writeFileSyncFn,
      renameSyncFn: options.renameSyncFn,
      chmodSyncFn: options.chmodSyncFn,
      createHashFn: options.createHashFn,
      copyFileSyncFn: options.copyFileSyncFn,
      unlinkSyncFn: options.unlinkSyncFn,
      mkdtempSyncFn: options.mkdtempSyncFn,
    });
  }

  if (!installResult.ok) {
    return {
      success: false,
      action: "error",
      channel,
      install_path: channelInfo.installPath || null,
      current_version: currentVersion,
      latest_version: latest.version,
      latest_source: latest.source,
      update_available: true,
      base_url_migrated: baseUrlMigrated,
      install: installResult,
      manual: manualInstallHints(channel, latest.version),
      error: installResult.error || "update failed",
    };
  }

  // Keep editor skill trees in sync with the package skill/ after CLI upgrade.
  // link installs re-resolve via package path; copy installs are refreshed.
  let skillSync = null;
  if (!options.skipSkillSync) {
    try {
      const skill = require("./skill");
      skillSync = skill.syncAllSkills({
        includeDiscovered: true,
        cliVersion: latest.version,
        proc: options.proc || process,
      });
    } catch (err) {
      skillSync = {
        success: false,
        error: err && err.message ? err.message : String(err),
      };
    }
  }

  return {
    success: true,
    action: "updated",
    channel,
    install_path: channelInfo.installPath || null,
    current_version: currentVersion,
    latest_version: latest.version,
    latest_source: latest.source,
    previous_version: currentVersion,
    update_available: false,
    base_url_migrated: baseUrlMigrated,
    skill_sync: skillSync,
    install: {
      method: installResult.method,
      command: installResult.command,
      asset: installResult.asset || null,
      checksum_ok: installResult.checksum_ok ?? null,
    },
    message: `updated ${currentVersion} → ${latest.version} via ${channel}`,
  };
}

/**
 * Background self-update hook for normal CLI invocations.
 *
 * - Default enabled (config auto_update / env / flag).
 * - Throttled to once per interval (default 24h).
 * - Never throws; never writes to stdout (caller may log to stderr).
 * - Skips update/upgrade commands, source/unknown channels, and disabled config.
 *
 * @param {{
 *   currentVersion: string,
 *   flags?: { noAutoUpdate?: boolean },
 *   commandName?: string,
 *   now?: number,
 *   intervalMs?: number,
 *   fetchImpl?: typeof fetch,
 *   spawnSyncFn?: typeof spawnSync,
 *   proc?: NodeJS.Process,
 *   onUpdated?: (result: object) => void,
 *   skipCommands?: string[],
 * }} options
 */
async function maybeAutoUpdate(options) {
  const skipCommands = options.skipCommands || ["update", "upgrade"];
  const commandName = options.commandName || "";
  if (commandName && skipCommands.includes(commandName)) {
    return { skipped: true, reason: "command" };
  }

  const enabled = resolveAutoUpdate(options.flags || {});
  if (!enabled.value) {
    return { skipped: true, reason: "disabled", source: enabled.source };
  }

  const channelInfo = detectInstallChannel(options.proc || process);
  if (channelInfo.channel === "source" || channelInfo.channel === "unknown") {
    // Still heal legacy domain config; do not hit the network for self-update.
    try {
      migrateLegacyBaseUrl();
    } catch {
      /* ignore */
    }
    return {
      skipped: true,
      reason: "channel",
      channel: channelInfo.channel,
    };
  }

  const now = typeof options.now === "number" ? options.now : Date.now();
  const intervalMs =
    typeof options.intervalMs === "number" && options.intervalMs >= 0
      ? options.intervalMs
      : DEFAULT_AUTO_UPDATE_INTERVAL_MS;
  if (!shouldRunAutoUpdateCheck(now, intervalMs)) {
    return { skipped: true, reason: "throttled" };
  }

  // Record attempt time first so a slow/failing network does not re-run every invoke.
  try {
    saveUpdateState({
      ...loadUpdateState(),
      last_check_at: now,
      last_result: "checking",
    });
  } catch {
    /* ignore state write failures */
  }

  let result;
  try {
    result = await performUpdate({
      currentVersion: options.currentVersion,
      checkOnly: false,
      force: false,
      proc: options.proc,
      fetchImpl: options.fetchImpl,
      spawnSyncFn: options.spawnSyncFn,
      fetchTimeoutMs: AUTO_UPDATE_FETCH_TIMEOUT_MS,
      writeFileSyncFn: options.writeFileSyncFn,
      renameSyncFn: options.renameSyncFn,
      chmodSyncFn: options.chmodSyncFn,
      createHashFn: options.createHashFn,
      copyFileSyncFn: options.copyFileSyncFn,
      unlinkSyncFn: options.unlinkSyncFn,
      mkdtempSyncFn: options.mkdtempSyncFn,
    });
  } catch (err) {
    try {
      saveUpdateState({
        last_check_at: now,
        last_result: "error",
        last_version: stripVersionPrefix(options.currentVersion),
      });
    } catch {
      /* ignore */
    }
    return {
      skipped: false,
      success: false,
      action: "error",
      error: err && err.message ? err.message : String(err),
    };
  }

  try {
    saveUpdateState({
      last_check_at: now,
      last_result: result.action || (result.success ? "ok" : "error"),
      last_version: result.latest_version || result.current_version || null,
    });
  } catch {
    /* ignore */
  }

  if (result.action === "updated" && typeof options.onUpdated === "function") {
    try {
      options.onUpdated(result);
    } catch {
      /* ignore notify failures */
    }
  }

  return { skipped: false, ...result };
}

module.exports = {
  NPM_PACKAGE,
  RELEASE_REPO,
  LEGACY_BASE_HOSTS,
  DEFAULT_AUTO_UPDATE_INTERVAL_MS,
  AUTO_UPDATE_FETCH_TIMEOUT_MS,
  parseSemver,
  compareSemver,
  stripVersionPrefix,
  detectInstallChannel,
  migrateLegacyBaseUrl,
  updateStatePath,
  loadUpdateState,
  saveUpdateState,
  shouldRunAutoUpdateCheck,
  fetchLatestNpmVersion,
  fetchLatestGithubVersion,
  fetchLatestVersion,
  resolvePlatformAsset,
  updateViaNpm,
  updateViaBinary,
  manualInstallHints,
  performUpdate,
  maybeAutoUpdate,
};
