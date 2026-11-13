const { test } = require("node:test");
const assert = require("node:assert/strict");

const {
  printJson,
  printError,
  ok,
  fail,
  setActiveMeta,
  UsageError,
} = require("../src/lib/output");
const {
  ConfigError,
  AuthError,
  NetworkError,
  exitCodeOf,
} = require("../src/lib/errors");

function capture(stream, fn) {
  const original = stream.write;
  let out = "";
  stream.write = (chunk) => {
    out += chunk;
    return true;
  };
  try {
    fn();
  } finally {
    stream.write = original;
  }
  return out;
}

test("ok writes success envelope to stdout", () => {
  setActiveMeta({ command: "models", cli_version: "9.9.9" });
  const out = capture(process.stdout, () => ok({ a: 1 }));
  const env = JSON.parse(out);
  assert.equal(env.ok, true);
  assert.deepEqual(env.data, { a: 1 });
  assert.equal(env.meta.command, "models");
  assert.equal(env.meta.cli_version, "9.9.9");
  assert.equal(env.meta.contract_version, 1);
});

test("printJson adapter wraps payload under data", () => {
  setActiveMeta({ command: "tokens", cli_version: "1.0.0" });
  const out = capture(process.stdout, () => printJson({ token_id: 42 }));
  const env = JSON.parse(out);
  assert.equal(env.ok, true);
  assert.equal(env.data.token_id, 42);
});

test("printJson --pretty indents with two spaces", () => {
  setActiveMeta({ command: "x", cli_version: "0" });
  const out = capture(process.stdout, () => printJson({ a: 1 }, { pretty: true }));
  assert.match(out, /^\{\n  "ok": true,/);
});

test("fail writes error envelope to stderr and returns exit code", () => {
  setActiveMeta({ command: "models", cli_version: "1.0.0" });
  const err = new AuthError("access token is not configured", {
    subtype: "token_missing",
    recovery: { command: "omnimux", args: ["login", "--no-wait"] },
  });
  let code;
  const out = capture(process.stderr, () => {
    code = fail(err);
  });
  assert.equal(code, 2);
  const env = JSON.parse(out);
  assert.equal(env.ok, false);
  assert.equal(env.error.type, "auth");
  assert.equal(env.error.subtype, "token_missing");
  assert.equal(env.error.recovery.command, "omnimux");
  assert.ok(!("cause" in env.error));
});

test("printError sanitizes secrets in message", () => {
  setActiveMeta({ command: "x", cli_version: "0" });
  const err = new Error("failed with key sk-abcdef123456 in body");
  const out = capture(process.stderr, () => {
    const code = printError(err);
    assert.equal(code, 5); // untyped → internal
  });
  assert.ok(!out.includes("sk-abcdef123456"));
  assert.match(out, /sk-<REDACTED>/);
});

test("ConfigError exit 2", () => {
  assert.equal(exitCodeOf(new ConfigError("missing")), 2);
});

test("UsageError exit 3", () => {
  assert.equal(new UsageError("bad args").exitCode, 3);
  assert.equal(new UsageError("bad").type, "usage");
});

test("NetworkError exit 4 and retryable", () => {
  const e = new NetworkError("timeout", { subtype: "timeout" });
  assert.equal(e.exitCode, 4);
  assert.equal(e.retryable, true);
});
