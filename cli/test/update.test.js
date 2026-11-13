const { test, beforeEach, afterEach } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");

const config = require("../src/lib/config");
const update = require("../src/lib/update");

let dir;
const touchedEnv = [
  "OMNIMUX_CONFIG_DIR",
  "OMNIMUX_BASE_URL",
  "OMNIMUX_USER_ID",
  "OMNIMUX_AUTO_UPDATE",
];
let savedEnv;

beforeEach(() => {
  savedEnv = {};
  for (const key of touchedEnv) {
    savedEnv[key] = process.env[key];
    delete process.env[key];
  }
  dir = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-update-"));
  process.env.OMNIMUX_CONFIG_DIR = dir;
});

afterEach(() => {
  for (const key of touchedEnv) {
    if (savedEnv[key] === undefined) delete process.env[key];
    else process.env[key] = savedEnv[key];
  }
  fs.rmSync(dir, { recursive: true, force: true });
});

test("compareSemver orders versions", () => {
  assert.equal(update.compareSemver("0.2.2", "0.2.3"), -1);
  assert.equal(update.compareSemver("0.2.3", "0.2.3"), 0);
  assert.equal(update.compareSemver("cli-v0.3.0", "v0.2.9"), 1);
  assert.equal(update.parseSemver("not-a-version"), null);
});

test("detectInstallChannel: npm global path", () => {
  const info = update.detectInstallChannel({
    argv: [
      "/usr/local/bin/node",
      "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
      "update",
    ],
    execPath: "/usr/local/bin/node",
  });
  assert.equal(info.channel, "npm");
  assert.match(info.installPath, /@omnimux\/cli/);
});

test("detectInstallChannel: nvm bin shim realpath → npm", () => {
  // Simulate nvm layout: HOME/bin/omnimux → lib/node_modules/@omnimux/cli/src/bin.js
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-nvm-"));
  try {
    const pkgBin = path.join(root, "lib", "node_modules", "@omnimux", "cli", "src");
    fs.mkdirSync(pkgBin, { recursive: true });
    const realBin = path.join(pkgBin, "bin.js");
    fs.writeFileSync(realBin, "#!/usr/bin/env node\n");
    const shimDir = path.join(root, "bin");
    fs.mkdirSync(shimDir, { recursive: true });
    const shim = path.join(shimDir, "omnimux");
    fs.symlinkSync(realBin, shim);
    const info = update.detectInstallChannel({
      argv: ["/usr/local/bin/node", shim, "update"],
      execPath: "/usr/local/bin/node",
    });
    assert.equal(info.channel, "npm");
    assert.match(info.installPath.replace(/\\/g, "/"), /@omnimux\/cli/);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("detectInstallChannel: compiled binary", () => {
  const info = update.detectInstallChannel({
    argv: ["/usr/local/bin/omnimux", "update"],
    execPath: "/usr/local/bin/omnimux",
  });
  assert.equal(info.channel, "binary");
  assert.equal(info.installPath, "/usr/local/bin/omnimux");
});

test("detectInstallChannel: source checkout", () => {
  const info = update.detectInstallChannel({
    argv: [
      "/home/dev/.bun/bin/bun",
      "/home/dev/src/omnimux/cli/src/bin.js",
      "help",
    ],
    execPath: "/home/dev/.bun/bin/bun",
  });
  assert.equal(info.channel, "source");
});

test("migrateLegacyBaseUrl rewrites geminix.cc to official default", () => {
  config.saveConfig({ base_url: "https://geminix.cc", user_id: "1" });
  const migrated = update.migrateLegacyBaseUrl();
  assert.deepEqual(migrated, {
    from: "https://geminix.cc",
    to: config.OMNIMUX_DEFAULT_BASE_URL,
  });
  assert.equal(config.loadConfig().base_url, config.OMNIMUX_DEFAULT_BASE_URL);
  assert.equal(config.loadConfig().user_id, "1");
});

test("migrateLegacyBaseUrl leaves non-legacy hosts alone", () => {
  config.saveConfig({ base_url: "https://self-hosted.example.com" });
  assert.equal(update.migrateLegacyBaseUrl(), null);
  assert.equal(config.loadConfig().base_url, "https://self-hosted.example.com");
});

function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
    arrayBuffer: async () => Buffer.from(JSON.stringify(body)),
  };
}

test("performUpdate --check reports update available without installing", async () => {
  const fetchImpl = async (url) => {
    if (String(url).includes("registry.npmjs.org")) {
      return jsonResponse({ version: "9.9.9" });
    }
    throw new Error(`unexpected fetch: ${url}`);
  };
  const result = await update.performUpdate({
    currentVersion: "0.2.3",
    checkOnly: true,
    skipMigrate: true,
    fetchImpl,
    proc: {
      argv: [
        "/usr/local/bin/node",
        "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
      ],
      execPath: "/usr/local/bin/node",
    },
  });
  assert.equal(result.success, true);
  assert.equal(result.action, "check_only");
  assert.equal(result.channel, "npm");
  assert.equal(result.current_version, "0.2.3");
  assert.equal(result.latest_version, "9.9.9");
  assert.equal(result.update_available, true);
  assert.ok(result.manual && result.manual.primary.includes("@omnimux/cli@9.9.9"));
});

test("performUpdate already_latest when versions match", async () => {
  const fetchImpl = async () => jsonResponse({ version: "0.2.3" });
  const result = await update.performUpdate({
    currentVersion: "0.2.3",
    checkOnly: false,
    skipMigrate: true,
    fetchImpl,
    proc: {
      argv: [
        "/usr/local/bin/node",
        "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
      ],
      execPath: "/usr/local/bin/node",
    },
  });
  assert.equal(result.success, true);
  assert.equal(result.action, "already_latest");
  assert.equal(result.update_available, false);
});

test("performUpdate npm channel installs via npm when outdated", async () => {
  const fetchImpl = async () => jsonResponse({ version: "0.3.0" });
  const calls = [];
  const spawnSyncFn = (cmd, args) => {
    calls.push([cmd, ...args]);
    if (cmd === "npm") {
      return { status: 0, stdout: "added 1 package", stderr: "", error: null };
    }
    return { status: 1, stdout: "", stderr: "not found", error: null };
  };
  const result = await update.performUpdate({
    currentVersion: "0.2.3",
    fetchImpl,
    spawnSyncFn,
    skipSkillSync: true, // The simulated install must not sync real user/project skill trees.
    skipMigrate: true,
    proc: {
      argv: [
        "/usr/local/bin/node",
        "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
      ],
      execPath: "/usr/local/bin/node",
    },
  });
  assert.equal(result.success, true);
  assert.equal(result.action, "updated");
  assert.equal(result.latest_version, "0.3.0");
  assert.equal(result.install.method, "npm");
  assert.deepEqual(calls[0], ["npm", "install", "-g", "@omnimux/cli@0.3.0"]);
});

test("performUpdate source channel requires manual install", async () => {
  const fetchImpl = async () => jsonResponse({ version: "0.3.0" });
  const result = await update.performUpdate({
    currentVersion: "0.2.3",
    fetchImpl,
    skipMigrate: true,
    proc: {
      argv: ["/usr/local/bin/node", "/repo/cli/src/bin.js"],
      execPath: "/usr/local/bin/node",
    },
  });
  assert.equal(result.success, false);
  assert.equal(result.action, "manual_required");
  assert.equal(result.channel, "source");
  assert.ok(result.manual.primary.includes("npm install"));
});

test("performUpdate migrates legacy base_url during check", async () => {
  config.saveConfig({ base_url: "https://www.geminix.cc" });
  const fetchImpl = async () => jsonResponse({ version: "0.2.3" });
  const result = await update.performUpdate({
    currentVersion: "0.2.3",
    checkOnly: true,
    fetchImpl,
    proc: {
      argv: [
        "/usr/local/bin/node",
        "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
      ],
      execPath: "/usr/local/bin/node",
    },
  });
  assert.equal(result.success, true);
  assert.deepEqual(result.base_url_migrated, {
    from: "https://www.geminix.cc",
    to: config.OMNIMUX_DEFAULT_BASE_URL,
  });
  assert.equal(config.loadConfig().base_url, config.OMNIMUX_DEFAULT_BASE_URL);
});

test("updateViaBinary writes asset with checksum verification", async () => {
  const assetBody = Buffer.from("fake-binary-content");
  const hash = crypto.createHash("sha256").update(assetBody).digest("hex");
  const dest = path.join(dir, "omnimux-bin");
  fs.writeFileSync(dest, "old");

  const platform = update.resolvePlatformAsset();
  if (platform.error) {
    // Skip body on unsupported CI arch; detection still unit-tested above.
    return;
  }

  const fetchImpl = async (url) => {
    const u = String(url);
    if (u.endsWith("/SHA256SUMS")) {
      return {
        ok: true,
        status: 200,
        text: async () => `${hash}  ${platform.asset}\n`,
        json: async () => ({}),
        arrayBuffer: async () => Buffer.alloc(0),
      };
    }
    if (u.includes(platform.asset)) {
      return {
        ok: true,
        status: 200,
        text: async () => "",
        json: async () => ({}),
        arrayBuffer: async () => assetBody,
      };
    }
    throw new Error(`unexpected fetch: ${u}`);
  };

  const result = await update.updateViaBinary("0.3.0", dest, { fetchImpl });
  assert.equal(result.ok, true);
  assert.equal(result.checksum_ok, true);
  assert.equal(fs.readFileSync(dest).toString(), "fake-binary-content");
});

const npmProc = {
  argv: [
    "/usr/local/bin/node",
    "/usr/local/lib/node_modules/@omnimux/cli/src/bin.js",
    "models",
  ],
  execPath: "/usr/local/bin/node",
};

test("maybeAutoUpdate skips when disabled via config", async () => {
  config.saveConfig({ auto_update: false });
  let fetched = false;
  const result = await update.maybeAutoUpdate({
    currentVersion: "0.2.3",
    commandName: "models",
    proc: npmProc,
    fetchImpl: async () => {
      fetched = true;
      return jsonResponse({ version: "9.9.9" });
    },
  });
  assert.equal(result.skipped, true);
  assert.equal(result.reason, "disabled");
  assert.equal(fetched, false);
});

test("maybeAutoUpdate skips update command to avoid recursion", async () => {
  let fetched = false;
  const result = await update.maybeAutoUpdate({
    currentVersion: "0.2.3",
    commandName: "update",
    proc: npmProc,
    fetchImpl: async () => {
      fetched = true;
      return jsonResponse({ version: "9.9.9" });
    },
  });
  assert.equal(result.skipped, true);
  assert.equal(result.reason, "command");
  assert.equal(fetched, false);
});

test("maybeAutoUpdate is throttled within interval", async () => {
  update.saveUpdateState({ last_check_at: 1_000_000, last_result: "ok" });
  let fetched = false;
  const result = await update.maybeAutoUpdate({
    currentVersion: "0.2.3",
    commandName: "models",
    now: 1_000_000 + 60_000, // 1 minute later
    intervalMs: update.DEFAULT_AUTO_UPDATE_INTERVAL_MS,
    proc: npmProc,
    fetchImpl: async () => {
      fetched = true;
      return jsonResponse({ version: "9.9.9" });
    },
  });
  assert.equal(result.skipped, true);
  assert.equal(result.reason, "throttled");
  assert.equal(fetched, false);
});

test("maybeAutoUpdate installs when outdated and not throttled", async () => {
  const calls = [];
  const spawnSyncFn = (cmd, args) => {
    calls.push([cmd, ...args]);
    return { status: 0, stdout: "ok", stderr: "", error: null };
  };
  let notified = null;
  const result = await update.maybeAutoUpdate({
    currentVersion: "0.2.3",
    commandName: "models",
    now: Date.now(),
    intervalMs: 0,
    skipSkillSync: true, // Keep automatic-update fixture effects inside its temporary config.
    proc: npmProc,
    fetchImpl: async () => jsonResponse({ version: "0.3.0" }),
    spawnSyncFn,
    onUpdated: (r) => {
      notified = r;
    },
  });
  assert.equal(result.skipped, false);
  assert.equal(result.action, "updated");
  assert.equal(result.latest_version, "0.3.0");
  assert.equal(notified && notified.action, "updated");
  assert.deepEqual(calls[0], ["npm", "install", "-g", "@omnimux/cli@0.3.0"]);
  const state = update.loadUpdateState();
  assert.equal(state.last_result, "updated");
});

test("maybeAutoUpdate skips source channel without network", async () => {
  let fetched = false;
  const result = await update.maybeAutoUpdate({
    currentVersion: "0.2.3",
    commandName: "models",
    intervalMs: 0,
    proc: {
      argv: ["/usr/local/bin/node", "/repo/cli/src/bin.js", "models"],
      execPath: "/usr/local/bin/node",
    },
    fetchImpl: async () => {
      fetched = true;
      return jsonResponse({ version: "9.9.9" });
    },
  });
  assert.equal(result.skipped, true);
  assert.equal(result.reason, "channel");
  assert.equal(fetched, false);
});
