/**
 * tokens create id-resolution and tokens exec --timeout tests.
 *
 * create: newer servers return data.id directly; older ones return
 * {success:true} with no data, so the CLI falls back to a key-masked list
 * call and resolves the id by name (newest match wins).
 *
 * exec: --timeout=<sec> controls the child-process timeout (default 300s);
 * invalid values are UsageError.
 */

const { test } = require("node:test");
const assert = require("node:assert/strict");
const childProcess = require("child_process");

const tokens = require("../src/commands/tokens");
const http = require("../src/lib/http");
const { UsageError } = require("../src/lib/output");

function captureStdout() {
  const original = process.stdout.write;
  let out = "";
  process.stdout.write = (chunk) => {
    out += chunk;
    return true;
  };
  return {
    restore() {
      process.stdout.write = original;
    },
    get text() {
      return out;
    },
  };
}

async function runCreate(api, flags = {}) {
  const cap = captureStdout();
  try {
    await tokens.run({ flags, args: ["create", "my-app"], auth: {}, api });
  } finally {
    cap.restore();
  }
  const env = JSON.parse(cap.text);
  // Agent RPC: business payload under data
  return env.data != null ? env.data : env;
}

test("create uses data.id from the server response without a fallback call", async () => {
  const calls = [];
  const api = async (method, urlPath, body) => {
    calls.push([method, urlPath]);
    return { success: true, data: { id: 42 } };
  };
  const out = await runCreate(api);
  assert.equal(out.token_id, 42);
  assert.match(out.next_step, /tokens copy 42/);
  assert.deepEqual(calls, [["POST", "/api/token/"]]);
});

test("create falls back to a masked list call and picks the newest matching id", async () => {
  const calls = [];
  const api = async (method, urlPath) => {
    calls.push([method, urlPath]);
    if (method === "POST") return { success: true, message: "" };
    return {
      success: true,
      data: [
        { id: 3, name: "other" },
        { id: 7, name: "my-app" },
        { id: 9, name: "my-app" },
      ],
    };
  };
  const out = await runCreate(api);
  assert.equal(out.token_id, 9);
  assert.match(out.next_step, /tokens copy 9/);
  assert.deepEqual(calls, [
    ["POST", "/api/token/"],
    ["GET", "/api/token/?p=0&page_size=20"],
  ]);
});

test("create outputs token_id null and a list-guided next_step when the id cannot be resolved", async () => {
  const api = async (method) => {
    if (method === "POST") return { success: true, message: "" };
    // Paginated shape, no token with the new name.
    return { success: true, data: { items: [{ id: 1, name: "other" }], total: 1 } };
  };
  const out = await runCreate(api);
  assert.equal(out.token_id, null);
  assert.match(out.next_step, /omnimux tokens`/);
});

async function runExec(flags, spawnSyncImpl, args = ["exec", "42", "echo", "__OMNIMUX_TOKEN_42__"]) {
  const originalFetchTokenKey = http.fetchTokenKey;
  const originalSpawnSync = childProcess.spawnSync;
  const originalExit = process.exit;
  const cap = captureStdout();
  let exitCode = null;
  http.fetchTokenKey = async () => "sk-testkey123456";
  childProcess.spawnSync = spawnSyncImpl;
  process.exit = (code) => {
    exitCode = code;
  };
  try {
    await tokens.run({
      flags,
      args,
      auth: {},
      api: async () => ({}),
    });
  } finally {
    http.fetchTokenKey = originalFetchTokenKey;
    childProcess.spawnSync = originalSpawnSync;
    process.exit = originalExit;
    cap.restore();
  }
  return { exitCode, stdout: cap.text };
}

test("exec rejects a non-numeric --timeout with UsageError", async () => {
  await assert.rejects(
    tokens.run({ flags: { timeout: "abc" }, args: ["exec", "42", "echo", "x"], auth: {}, api: async () => ({}) }),
    (err) => err instanceof UsageError && err.exitCode === 3
  );
});

test("exec rejects a non-positive --timeout with UsageError", async () => {
  await assert.rejects(
    tokens.run({ flags: { timeout: "0" }, args: ["exec", "42", "echo", "x"], auth: {}, api: async () => ({}) }),
    (err) => err instanceof UsageError && err.exitCode === 3
  );
});

test("exec defaults the child timeout to 300s", async () => {
  let seen = null;
  const { exitCode } = await runExec({}, (cmd, args, options) => {
    seen = { cmd, args, options };
    return { status: 0, stdout: "done\n", stderr: "", error: null, signal: null };
  });
  assert.equal(seen.options.timeout, 300000);
  assert.equal(exitCode, 0);
});

test("exec --timeout=<sec> is passed to the child process in milliseconds", async () => {
  let seen = null;
  await runExec({ timeout: "7" }, (cmd, args, options) => {
    seen = { cmd, args, options };
    return { status: 0, stdout: "done\n", stderr: "", error: null, signal: null };
  });
  assert.equal(seen.options.timeout, 7000);
});

test("exec substitutes placeholder per argv without shell-joining", async () => {
  let seen = null;
  await runExec(
    {},
    (cmd, args) => {
      seen = { cmd, args };
      return { status: 0, stdout: "ok\n", stderr: "", error: null, signal: null };
    },
    [
      "exec",
      "42",
      "curl",
      "-H",
      "Authorization: Bearer __OMNIMUX_TOKEN_42__",
      "https://example.com",
    ]
  );
  assert.equal(seen.cmd, "curl");
  assert.deepEqual(seen.args, [
    "-H",
    "Authorization: Bearer sk-testkey123456",
    "https://example.com",
  ]);
});

test("exec never leaks the real key into child output", async () => {
  const { stdout } = await runExec({ timeout: "5" }, () => ({
    status: 0,
    stdout: "token was sk-testkey123456\n",
    stderr: "",
    error: null,
    signal: null,
  }));
  assert.ok(!stdout.includes("sk-testkey123456"));
  assert.match(stdout, /sk-<REDACTED>/);
});
