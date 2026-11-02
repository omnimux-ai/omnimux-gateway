const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const http = require("node:http");
const { spawn } = require("node:child_process");

const BIN = path.join(__dirname, "..", "src", "bin.js");
const LEAVES = [
  "influencer.options", "influencer.character.create", "influencer.character.list",
  "influencer.character.get", "influencer.character.multiview", "influencer.video.create",
  "influencer.task.list", "influencer.task.get", "influencer.task.wait",
  "influencer.task.download", "influencer.request.get",
];
const WRITES = new Set(["influencer.character.create", "influencer.character.multiview", "influencer.video.create"]);
const signalTest = process.platform === "win32" ? test.skip : test;

function json(res, status, data) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(status < 400 ? { success: true, data } : { success: false, ...data }));
}
function task(overrides = {}) {
  return {
    request_id: "request-test-001", task_id: "task-created-001", kind: "character", status: "waiting",
    raw_status: "WAITING", artifact_status: "pending", recovery_expires_at: 0, ...overrides,
  };
}
function walk(dir) {
  if (!fs.existsSync(dir) || !fs.statSync(dir).isDirectory()) return [];
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((ent) => {
    const full = path.join(dir, ent.name);
    return ent.isDirectory() ? walk(full) : [full];
  });
}
function envelopes(result) {
  const output = result.code === 0 ? result.stdout : result.stderr;
  const lines = output.trim().split("\n").filter(Boolean);
  assert.ok(lines.length, `CLI omitted JSON envelope: ${JSON.stringify(result)}`);
  return JSON.parse(lines.at(-1));
}
function execute(args, env = {}, opts = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [...(opts.preload ? ["--require", opts.preload] : []), BIN, ...args], {
      cwd: path.join(__dirname, ".."),
      env: { ...process.env, OMNIMUX_AUTO_UPDATE: "0", OMNIMUX_ACCESS_TOKEN: "", OMNIMUX_USER_ID: "", ...env },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "", stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    if (opts.onSpawn) opts.onSpawn(child);
    const timer = setTimeout(() => { child.kill("SIGKILL"); reject(new Error("CLI process exceeded test deadline")); }, opts.timeout || 12000);
    child.once("error", (err) => { clearTimeout(timer); reject(err); });
    child.once("close", (code, signal) => { clearTimeout(timer); resolve({ code, signal, stdout, stderr }); });
  });
}
async function fixture(t, handler) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "omnimux-influencer-"));
  const config = path.join(root, "config");
  fs.mkdirSync(config, { mode: 0o700 });
  const calls = [];
  const serverErrors = [];
  const server = http.createServer(async (req, res) => {
    try {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const buffer = Buffer.concat(chunks);
      const call = { method: req.method, url: req.url, headers: req.headers, bytes: buffer };
      if (req.headers["content-type"]?.includes("application/json") && buffer.length) call.body = JSON.parse(buffer);
      calls.push(call);
      await handler(req, res, call, { root, config, calls });
    } catch (err) {
      serverErrors.push(err);
      if (!res.headersSent) json(res, 500, { code: "mock_failure", message: err.message });
      else res.destroy();
    }
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const baseUrl = `http://127.0.0.1:${server.address().port}`;
  const env = { OMNIMUX_CONFIG_DIR: config, OMNIMUX_BASE_URL: baseUrl, OMNIMUX_ACCESS_TOKEN: "test-access-token", OMNIMUX_USER_ID: "41" };
  t.after(async () => {
    server.closeAllConnections?.();
    await new Promise((resolve) => server.close(resolve));
    fs.rmSync(root, { recursive: true, force: true });
    assert.deepEqual(serverErrors, [], "local HTTP fixture encountered an assertion failure");
  });
  return { root, config, baseUrl, calls, env, run: (args, extraEnv, opts) => execute(args, { ...env, ...extraEnv }, opts) };
}
function assetPreload(fx) {
  const filename = path.join(fx.root, "asset-preload.cjs");
  // HTTPS-only production policy is still exercised. Only the transport of this reserved test host is redirected.
  fs.writeFileSync(filename, `const originalFetch = globalThis.fetch;\nglobalThis.fetch = (input, opts) => { const value = String(input); if (value.startsWith("https://assets.example.test/")) return originalFetch(${JSON.stringify(fx.baseUrl)} + "/storage/" + value.slice("https://assets.example.test/".length), opts); return originalFetch(input, opts); };\n`);
  return filename;
}

// These tests launch the real CLI rather than replacing its command context or API port.
test("influencer process: schema/help advertise every leaf and preserve risk metadata", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  const schema = await fx.run(["schema"]);
  assert.equal(schema.code, 0, schema.stderr);
  const catalog = envelopes(schema).data.commands;
  assert.deepEqual(catalog.filter((entry) => LEAVES.includes(entry.name)).map((entry) => entry.name).sort(), [...LEAVES].sort());
  for (const name of LEAVES) {
    const leaf = catalog.find((entry) => entry.name === name);
    assert.equal(leaf.auth, "full", name);
    assert.equal(leaf.risk, WRITES.has(name) ? "high-risk-write" : "read", name);
    assert.equal(leaf.dry_run, WRITES.has(name), name);
  }
  const selected = envelopes(await fx.run(["schema", "influencer.character.create"]));
  assert.equal(selected.data.command.risk, "high-risk-write");
  const help = envelopes(await fx.run(["help"]));
  for (const name of LEAVES) assert.ok(help.data.commands.some((entry) => entry.name === name), name);
  assert.equal(fx.calls.length, 0);
});

test("influencer process: every paid creation requires confirmation before auth/network", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  for (const args of [["character", "create"], ["character", "multiview", "parent-1"], ["video", "create"]]) {
    const result = await fx.run(["influencer", ...args], { OMNIMUX_ACCESS_TOKEN: "", OMNIMUX_USER_ID: "" });
    assert.equal(result.code, 10, result.stderr);
    const output = envelopes(result);
    assert.equal(output.error.type, "confirmation");
    assert.equal(output.error.subtype, "confirmation_required");
    assert.equal(output.error.risk, "high-risk-write");
    assert.equal(output.meta.command, ["influencer", args[0], args[1]].join("."));
  }
  assert.equal(fx.calls.length, 0);
});

test("influencer process: dry-run is offline, never reads secrets or creates a receipt", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  const image = path.join(fx.root, "style.png");
  const video = path.join(fx.root, "ref.mp4");
  fs.writeFileSync(image, "test-style"); fs.writeFileSync(video, "test-reference");
  const preload = path.join(fx.root, "offline-preload.cjs");
  const networkMarker = path.join(fx.root, "network-marker");
  const secretMarker = path.join(fx.root, "secret-marker");
  fs.writeFileSync(preload, `const fs = require("node:fs"); for (const method of ["existsSync", "readFileSync", "statSync"]) { const original = fs[method]; fs[method] = function(p, ...args) { if (String(p).endsWith("secrets.json")) { fs.writeFileSync(${JSON.stringify(secretMarker)}, method); throw new Error("TEST_SECRET_READ"); } return original.call(this, p, ...args); }; } globalThis.fetch = async () => { fs.writeFileSync(${JSON.stringify(networkMarker)}, "called"); throw new Error("TEST_NETWORK_CALL"); };\n`);
  for (const args of [
    ["character", "create", "--brief", "local preview", "--reference-image", image],
    ["character", "multiview", "parent-1"],
    ["video", "create", "--image", image, "--video", video],
  ]) {
    const result = await fx.run(["influencer", ...args, "--dry-run"], {
      OMNIMUX_ACCESS_TOKEN: "", OMNIMUX_USER_ID: "", OMNIMUX_AUTO_UPDATE: "1",
    }, { preload });
    assert.equal(result.code, 0, result.stderr);
    assert.equal(envelopes(result).data.dry_run, true);
  }
  assert.deepEqual(walk(fx.config), []);
  assert.equal(fs.existsSync(networkMarker), false);
  assert.equal(fs.existsSync(secretMarker), false);
  assert.equal(fx.calls.length, 0);
});

test("influencer process: local journal failure stops before upload or API", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  const blocked = path.join(fx.root, "not-a-directory"); fs.writeFileSync(blocked, "blocked");
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "style");
  const result = await fx.run(["influencer", "character", "create", "--yes", "--request-id", "journal-fail-001", "--reference-image", image], { OMNIMUX_CONFIG_DIR: blocked });
  assert.notEqual(result.code, 0);
  assert.equal(envelopes(result).ok, false);
  assert.notEqual(envelopes(result).error.subtype, "unknown_command");
  assert.equal(fx.calls.length, 0);
});

test("influencer process: character upload persists identity before network and sends final product body", async (t) => {
  let createBody;
  const fx = await fixture(t, (_req, res, call, state) => {
    assert.equal(call.headers.authorization, "Bearer test-access-token");
    assert.equal(call.headers["new-api-user"], "41");
    if (call.url === "/api/v1/files/upload/stream") {
      const journals = walk(state.config).filter((filename) => filename.endsWith(".json"));
      assert.ok(journals.some((filename) => fs.readFileSync(filename, "utf8").includes("create-upload-001")), "receipt must predate upload");
      json(res, 200, { file_id: "file_style", file_url: "https://files.example.test/style.png" });
    } else if (call.url === "/api/influencer/v1/characters" && call.method === "POST") {
      createBody = call.body;
      const journals = walk(state.config).filter((filename) => filename.endsWith(".json"));
      assert.ok(journals.some((filename) => fs.readFileSync(filename, "utf8").includes("https://files.example.test/style.png")), "uploaded reference must be durably saved before POST");
      json(res, 200, task({ request_id: call.body.request_id }));
    } else if (call.url.startsWith("/api/influencer/v1/requests/")) json(res, 404, { code: "creation_request_not_found", message: "not accepted" });
    else json(res, 500, { code: "unexpected_network" });
  });
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "local-image-bytes");
  const result = await fx.run(["influencer", "character", "create", "--yes", "--request-id", "create-upload-001", "--type", "normal", "--brief", "blue jacket", "--selection", '{"gender":["female"]}', "--reference-image", image]);
  assert.equal(result.code, 0, result.stderr);
  assert.deepEqual(createBody, { request_id: "create-upload-001", tier: "normal", selection: { gender: ["female"] }, brief: "blue jacket", image_url: "https://files.example.test/style.png" });
  const output = envelopes(result);
  assert.equal(output.meta.command, "influencer.character.create");
  assert.equal(output.data.request_id, "create-upload-001");
  assert.equal(output.data.task_id, "task-created-001");
  assert.equal(output.data.status, "waiting");
  const receipts = walk(fx.config).filter((filename) => filename.endsWith(".json"));
  assert.ok(receipts.length);
  for (let directory = path.dirname(receipts[0]); directory !== fx.config; directory = path.dirname(directory)) assert.equal(fs.statSync(directory).mode & 0o777, 0o700);
  for (const filename of receipts) {
    assert.equal(fs.statSync(filename).mode & 0o777, 0o600);
    assert.equal(fs.readFileSync(filename, "utf8").includes("test-access-token"), false);
  }
});

test("influencer process: video retains ordered references and explicit mode/resolution/aspect", async (t) => {
  let body;
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url === "/api/v1/files/upload/stream") json(res, 200, { file_url: `https://files.example.test/upload-${fx.calls.filter((c) => c.url === call.url).length}.png` });
    else if (call.url === "/api/influencer/v1/videos" && call.method === "POST") { body = call.body; json(res, 200, task({ request_id: body.request_id, kind: "video" })); }
    else if (call.url.startsWith("/api/influencer/v1/requests/")) json(res, 404, { code: "creation_request_not_found" });
    else json(res, 500, { code: "unexpected_network" });
  });
  const image = path.join(fx.root, "second.png"); fs.writeFileSync(image, "second-reference");
  const result = await fx.run(["influencer", "video", "create", "--yes", "--request-id", "video-order-001", "--character", "character-original", "--image", "https://files.example.test/first.png", "--image", image, "--video", "https://files.example.test/reference.mp4", "--mode", "replace", "--resolution", "720p", "--aspect-ratio", "16:9", "--prompt", "keep studio"]);
  assert.equal(result.code, 0, result.stderr);
  assert.deepEqual(body, { request_id: "video-order-001", character_task_id: "character-original", image_urls: ["https://files.example.test/first.png", "https://files.example.test/upload-1.png"], video_url: "https://files.example.test/reference.mp4", mode: "replace", resolution: "720p", aspect_ratio: "16:9", prompt: "keep studio" });
  assert.equal(envelopes(result).meta.command, "influencer.video.create");
});

test("influencer process: response loss is recovered by request without reupload or second POST", async (t) => {
  let accepted = false;
  const fx = await fixture(t, (req, res, call) => {
    if (call.url === "/api/v1/files/upload/stream") json(res, 200, { file_url: "https://files.example.test/stable.png" });
    else if (call.url === "/api/influencer/v1/characters" && call.method === "POST") { accepted = true; req.socket.destroy(); }
    else if (call.url === "/api/influencer/v1/requests/recover-request-001") {
      if (accepted) json(res, 200, task({ request_id: "recover-request-001" }));
      else json(res, 404, { code: "creation_request_not_found" });
    } else json(res, 500, { code: "unexpected_network" });
  });
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "original-image");
  const args = ["influencer", "character", "create", "--yes", "--request-id", "recover-request-001", "--reference-image", image];
  const first = await fx.run(args);
  assert.ok(accepted);
  if (first.code !== 0) assert.equal(envelopes(first).error.operation_id, "recover-request-001", "ambiguous submission must preserve recovery identity");
  fs.unlinkSync(image); // Recovery must remain possible after a local source disappears.
  const second = await fx.run(args);
  assert.equal(second.code, 0, second.stderr);
  assert.equal(envelopes(second).data.task_id, "task-created-001");
  assert.equal(fx.calls.filter((call) => call.url === "/api/v1/files/upload/stream").length, 1);
  assert.equal(fx.calls.filter((call) => call.method === "POST" && call.url === "/api/influencer/v1/characters").length, 1);
  assert.ok(fx.calls.filter((call) => call.url.includes("/requests/recover-request-001")).length >= 1);
});

test("influencer process: same request ID with changed intent or local bytes fails without network", async (t) => {
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url === "/api/v1/files/upload/stream") json(res, 200, { file_url: "https://files.example.test/original.png" });
    else if (call.method === "POST") json(res, 200, task({ request_id: "local-conflict-001" }));
    else json(res, 404, { code: "creation_request_not_found" });
  });
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "old-image");
  const args = ["influencer", "character", "create", "--yes", "--request-id", "local-conflict-001", "--brief", "first", "--reference-image", image];
  const created = await fx.run(args); assert.equal(created.code, 0, created.stderr);
  const before = fx.calls.length;
  const changedBrief = await fx.run(args.map((value) => value === "first" ? "second" : value));
  assert.notEqual(changedBrief.code, 0);
  assert.match(envelopes(changedBrief).error.subtype, /conflict/);
  assert.equal(fx.calls.length, before);
  fs.writeFileSync(image, "new-image");
  const changedFile = await fx.run(args);
  assert.notEqual(changedFile.code, 0);
  assert.match(envelopes(changedFile).error.subtype, /conflict/);
  assert.equal(fx.calls.length, before);
});

test("influencer process: non-overlapping input fields merge; overlap and hidden billing/model fields fail offline", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  const input = path.join(fx.root, "input.json"); fs.writeFileSync(input, JSON.stringify({ brief: "file brief", tier: "normal" }));
  const overlap = await fx.run(["influencer", "character", "create", "--dry-run", "--input", input, "--brief", "explicit brief"]);
  assert.equal(overlap.code, 3, overlap.stderr);
  fs.writeFileSync(input, JSON.stringify({ tier: "normal" }));
  const merged = await fx.run(["influencer", "character", "create", "--dry-run", "--input", input, "--brief", "supplemental brief", "--request-id", "input-merged-001"]);
  assert.equal(merged.code, 0, merged.stderr);
  assert.deepEqual(envelopes(merged).data.input, { tier: "normal", brief: "supplemental brief", request_id: "input-merged-001" });
  const sameTier = await fx.run(["influencer", "character", "create", "--dry-run", "--input", input, "--type", "normal", "--brief", "supplemental brief"]);
  assert.equal(sameTier.code, 3, sameTier.stderr);
  fs.writeFileSync(input, JSON.stringify({ tier: "normal", brief: "file brief", request_id: "input-merged-001" }));
  const sameRequest = await fx.run(["influencer", "character", "create", "--dry-run", "--input", input, "--request-id", "input-merged-001"]);
  assert.equal(sameRequest.code, 3, sameRequest.stderr);
  for (const bad of [{ model: "private-model" }, { group: "private-group" }, { duration: 10 }]) {
    fs.writeFileSync(input, JSON.stringify({ ...bad, image_urls: ["https://files.example.test/i.png"], video_url: "https://files.example.test/v.mp4" }));
    const result = await fx.run(["influencer", "video", "create", "--dry-run", "--input", input]);
    assert.equal(result.code, 3, result.stderr);
  }
  assert.equal(fx.calls.length, 0);
});

test("influencer process: read endpoints preserve owner auth, pagination and dotted meta", async (t) => {
  const fx = await fixture(t, (_req, res, call) => {
    assert.equal(call.headers.authorization, "Bearer test-access-token");
    if (call.url.includes("/requests/")) json(res, 200, task());
    else if (call.url.includes("/tasks/task-1")) json(res, 200, task({ task_id: "task-1" }));
    else if (call.url.includes("/characters/parent-1")) json(res, 200, task({ task_id: "parent-1" }));
    else json(res, 200, { items: [], page: 2, page_size: 7 });
  });
  for (const [args, meta] of [
    [["options"], "influencer.options"],
    [["character", "list", "--page", "2", "--page-size=7"], "influencer.character.list"],
    [["character", "get", "parent-1"], "influencer.character.get"],
    [["task", "list", "--page", "2", "--page-size", "7", "--kind", "video", "--status", "running"], "influencer.task.list"],
    [["task", "get", "task-1"], "influencer.task.get"],
    [["request", "get", "request-test-001"], "influencer.request.get"],
  ]) {
    const result = await fx.run(["influencer", ...args]);
    assert.equal(result.code, 0, result.stderr);
    assert.equal(envelopes(result).meta.command, meta);
  }
  const list = fx.calls.find((call) => call.url.startsWith("/api/influencer/v1/characters?"));
  const query = new URL(list.url, fx.baseUrl).searchParams;
  assert.equal(query.get("page"), "2"); assert.equal(query.get("page_size"), "7");
  const history = fx.calls.find((call) => call.url.startsWith("/api/influencer/v1/tasks?"));
  const historyQuery = new URL(history.url, fx.baseUrl).searchParams;
  assert.equal(historyQuery.get("page_size"), "7"); assert.equal(historyQuery.get("kind"), "video"); assert.equal(historyQuery.get("status"), "running");
});

test("influencer process: wait timeout and failed generation preserve IDs and do not create", async (t) => {
  const fx = await fixture(t, (_req, res, call) => {
    assert.equal(call.method, "GET");
    if (call.url.endsWith("/failed-task")) json(res, 200, task({ task_id: "failed-task", status: "failed", raw_status: "FAILURE", artifact_status: "unavailable" }));
    else json(res, 200, task({ task_id: "running-task", status: "running", raw_status: "IN_PROGRESS" }));
  });
  const timed = await fx.run(["influencer", "task", "wait", "running-task", "--timeout", "1"]);
  assert.equal(timed.code, 1, timed.stderr);
  const timeout = envelopes(timed);
  assert.equal(timeout.error.type, "api"); assert.equal(timeout.error.subtype, "wait_timeout"); assert.equal(timeout.error.task_id, "running-task");
  assert.equal(timeout.meta.command, "influencer.task.wait");
  const failed = await fx.run(["influencer", "task", "wait", "failed-task"]);
  assert.equal(failed.code, 1, failed.stderr);
  assert.equal(envelopes(failed).error.task_id, "failed-task");
  assert.notEqual(envelopes(failed).error.subtype, "wait_timeout");
});

test("influencer process: existing output is refused before API or storage access", async (t) => {
  const fx = await fixture(t, (_req, res) => json(res, 500, { code: "unexpected_network" }));
  const output = path.join(fx.root, "existing.png"); fs.writeFileSync(output, "preserve-me");
  const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output]);
  assert.notEqual(result.code, 0);
  assert.notEqual(envelopes(result).error.subtype, "unknown_command");
  assert.equal(fs.readFileSync(output, "utf8"), "preserve-me");
  assert.equal(fx.calls.length, 0);
});

function artifacts(url, type = "image/png") {
  return { task_id: "ready-task", artifact_status: "ready", artifacts: [{ key: "image-0", type: "image", mime_type: type, content_url: url }] };
}

test("influencer process: download streams complete media with no credential on storage", async (t) => {
  const bytes = Buffer.from("\x89PNG\r\n\x1a\nlocal-media-body", "binary");
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url === "/api/influencer/v1/tasks/ready-task/artifacts") {
      assert.equal(call.headers.authorization, "Bearer test-access-token");
      json(res, 200, artifacts("https://assets.example.test/good.png?signature=private"));
    } else if (call.url.startsWith("/storage/")) {
      assert.equal(call.headers.authorization, undefined); assert.equal(call.headers["new-api-user"], undefined);
      res.writeHead(200, { "content-type": "image/png", "content-length": bytes.length }); res.end(bytes);
    } else if (call.url === "/api/influencer/v1/tasks/ready-task") json(res, 200, task({ task_id: "ready-task", status: "succeeded", artifact_status: "ready" }));
    else json(res, 500, { code: "unexpected_network" });
  });
  const output = path.join(fx.root, "downloaded.png");
  const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output], {}, { preload: assetPreload(fx) });
  assert.equal(result.code, 0, result.stderr);
  assert.deepEqual(fs.readFileSync(output), bytes);
  assert.equal(envelopes(result).meta.command, "influencer.task.download");
  assert.equal(result.stdout.includes("signature=private"), false);
  assert.deepEqual(fs.readdirSync(fx.root).filter((name) => name.includes(".partial") || name.includes(".part-") || name.includes(".tmp")), []);
});

test("influencer process: rejected or interrupted downloads leave no completed or partial file", async (t) => {
  let scenario = "wrong-type";
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url.endsWith("/artifacts")) json(res, 200, artifacts(`https://assets.example.test/${scenario}.png?secret=storage-secret`));
    else if (call.url === "/api/influencer/v1/tasks/ready-task") json(res, 200, task({ status: "succeeded", artifact_status: "ready" }));
    else if (call.url.startsWith("/storage/wrong-type")) { res.writeHead(200, { "content-type": "text/html" }); res.end("<html>expired</html>"); }
    else if (call.url.startsWith("/storage/expired")) { res.writeHead(403, { "content-type": "text/plain" }); res.end("expired signed URL"); }
    else if (call.url.startsWith("/storage/interrupted")) { res.writeHead(200, { "content-type": "image/png", "content-length": 20000 }); res.write("incomplete"); setTimeout(() => res.destroy(), 30); }
    else json(res, 500, { code: "unexpected_network" });
  });
  const preload = assetPreload(fx);
  for (scenario of ["wrong-type", "expired", "interrupted"]) {
    const output = path.join(fx.root, `${scenario}.png`);
    const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output], {}, { preload });
    assert.notEqual(result.code, 0, scenario);
    assert.equal(envelopes(result).ok, false);
    assert.ok(fx.calls.some((call) => call.url.startsWith(`/storage/${scenario}`)), "failure must exercise storage transport");
    assert.equal(fs.existsSync(output), false, scenario);
    assert.equal(result.stderr.includes("storage-secret"), false);
    assert.deepEqual(fs.readdirSync(fx.root).filter((name) => name.startsWith(`${scenario}.png`)), [], scenario);
  }
});

test("influencer process: multiview references its parent, defaults stay product-facing, IDs may be generated", async (t) => {
  const submitted = [];
  const fx = await fixture(t, (_req, res, call) => {
    submitted.push(call);
    json(res, 200, task({ request_id: call.body.request_id, kind: call.url.endsWith("/multiview") ? "multiview" : "video" }));
  });
  const multiview = await fx.run(["influencer", "character", "multiview", "parent-source", "--request-id", "multiview-test-001", "--yes"]);
  assert.equal(multiview.code, 0, multiview.stderr);
  assert.equal(envelopes(multiview).meta.command, "influencer.character.multiview");
  assert.equal(submitted[0].url, "/api/influencer/v1/characters/parent-source/multiview");
  assert.deepEqual(submitted[0].body, { request_id: "multiview-test-001" });
  const video = await fx.run(["influencer", "video", "create", "--character", "parent-source", "--video-task", "reference-source", "--yes"]);
  assert.equal(video.code, 0, video.stderr);
  assert.equal(submitted[1].url, "/api/influencer/v1/videos");
  assert.equal(submitted[1].body.mode, "motion"); assert.equal(submitted[1].body.resolution, "480p");
  assert.equal(submitted[1].body.aspect_ratio, undefined, "omitted aspect must remain server-inspected");
  assert.equal(submitted[1].body.video_task_id, "reference-source");
  assert.match(submitted[1].body.request_id, /^[A-Za-z0-9._:-]{8,128}$/);
  assert.equal(envelopes(video).data.request_id, submitted[1].body.request_id);
});

test("influencer process: definitive recovery 404 retries only the exact persisted body without upload", async (t) => {
  const bodies = [];
  const fx = await fixture(t, (req, res, call) => {
    if (call.url === "/api/v1/files/upload/stream") json(res, 200, { file_url: "https://files.example.test/persisted.png" });
    else if (call.url.endsWith("/characters") && call.method === "POST") {
      bodies.push(call.body);
      if (bodies.length === 1) req.socket.destroy();
      else json(res, 200, task({ request_id: "retry-exact-body-001" }));
    } else if (call.url.endsWith("/requests/retry-exact-body-001")) json(res, 404, { code: "creation_request_not_found" });
    else json(res, 500, { code: "unexpected_network" });
  });
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "original-image");
  const args = ["influencer", "character", "create", "--request-id", "retry-exact-body-001", "--reference-image", image, "--yes"];
  const first = await fx.run(args); assert.notEqual(first.code, 0);
  fs.unlinkSync(image);
  const second = await fx.run(args); assert.equal(second.code, 0, second.stderr);
  assert.equal(bodies.length, 2); assert.deepEqual(bodies[1], bodies[0]);
  assert.equal(fx.calls.filter((call) => call.url === "/api/v1/files/upload/stream").length, 1);
  const recoveryIndex = fx.calls.findIndex((call) => call.url.endsWith("/requests/retry-exact-body-001"));
  assert.ok(recoveryIndex > 0 && recoveryIndex < fx.calls.length - 1);
});

test("influencer process: concurrent identical local requests hold one upload/submission lock", async (t) => {
  let releaseUpload, seenUpload;
  const uploaded = new Promise((resolve) => { seenUpload = resolve; });
  const release = new Promise((resolve) => { releaseUpload = resolve; });
  const fx = await fixture(t, async (_req, res, call) => {
    if (call.url === "/api/v1/files/upload/stream") { seenUpload(); await release; json(res, 200, { file_url: "https://files.example.test/locked.png" }); }
    else if (call.method === "POST") json(res, 200, task({ request_id: "concurrent-local-001" }));
    else json(res, 500, { code: "unexpected_network" });
  });
  const image = path.join(fx.root, "style.png"); fs.writeFileSync(image, "style-image");
  const args = ["influencer", "character", "create", "--request-id", "concurrent-local-001", "--reference-image", image, "--yes"];
  const first = fx.run(args);
  let uploadDeadline;
  try {
    await Promise.race([uploaded, new Promise((_, reject) => { uploadDeadline = setTimeout(() => reject(new Error("first process never uploaded")), 5000); })]);
    const second = await fx.run(args);
    assert.notEqual(second.code, 0);
    assert.equal(envelopes(second).error.subtype, "creation_request_in_progress");
    assert.equal(envelopes(second).error.operation_id, "concurrent-local-001");
  } finally { clearTimeout(uploadDeadline); releaseUpload(); }
  const finished = await first; assert.equal(finished.code, 0, finished.stderr);
  assert.equal(fx.calls.filter((call) => call.url === "/api/v1/files/upload/stream").length, 1);
  assert.equal(fx.calls.filter((call) => call.method === "POST" && call.url.endsWith("/characters")).length, 1);
  assert.deepEqual(walk(fx.config).filter((file) => file.endsWith(".lock")), []);
});

signalTest("influencer process: killed submission recovers its stale lock and accepted task", async (t) => {
  let child, accepted = false;
  const fx = await fixture(t, (req, res, call) => {
    if (call.method === "POST") { accepted = true; child.kill("SIGKILL"); req.socket.destroy(); }
    else if (call.url.endsWith("/requests/killed-local-001") && accepted) json(res, 200, task({ request_id: "killed-local-001" }));
    else json(res, 500, { code: "unexpected_network" });
  });
  const args = ["influencer", "character", "create", "--request-id", "killed-local-001", "--brief", "before interruption", "--yes"];
  const first = await fx.run(args, {}, { onSpawn: (value) => { child = value; } });
  assert.equal(first.signal, "SIGKILL");
  assert.equal(walk(fx.config).filter((file) => file.endsWith(".lock")).length, 1, "forced termination must leave the genuine stale lock");
  const second = await fx.run(args); assert.equal(second.code, 0, second.stderr);
  assert.equal(envelopes(second).data.task_id, "task-created-001");
  assert.equal(fx.calls.filter((call) => call.method === "POST").length, 1);
  assert.deepEqual(walk(fx.config).filter((file) => file.endsWith(".lock")), []);
});

test("influencer process: stable service failures retain operation identity without generic success", async (t) => {
  let status, code;
  const fx = await fixture(t, (_req, res) => json(res, status, { code, message: "service refused this request" }));
  const examples = [
    [403, "ACCESS_TOKEN_SCOPE_DENIED", "auth", "scope_required", 2],
    [403, "influencer_creation_scope_denied", "auth", "scope_required", 2],
    [409, "creation_request_conflict", "api", "creation_request_conflict", 1],
    [410, "creation_request_expired", "api", "creation_request_expired", 1],
    [503, "influencer_creation_disabled", "api", "influencer_creation_disabled", 1],
  ];
  for (const [responseStatus, responseCode, type, subtype, exit] of examples) {
    status = responseStatus; code = responseCode;
    const id = `service-error-${status}-${responseCode}`;
    const result = await fx.run(["influencer", "character", "create", "--brief", "failure example", "--request-id", id, "--yes"]);
    assert.equal(result.code, exit, result.stderr);
    const output = envelopes(result);
    assert.equal(output.error.type, type); assert.equal(output.error.subtype, subtype);
    assert.equal(output.error.operation_id, id);
    assert.equal(output.meta.command, "influencer.character.create");
  }
});

test("influencer process: expired recovery never reopens or reposts an accepted operation", async (t) => {
  let accepted = false;
  const fx = await fixture(t, (req, res, call) => {
    if (call.method === "POST") { accepted = true; req.socket.destroy(); }
    else json(res, 410, { code: "creation_request_expired", message: "recovery expired" });
  });
  const args = ["influencer", "character", "create", "--request-id", "expired-request-001", "--brief", "old task", "--yes"];
  const first = await fx.run(args); assert.notEqual(first.code, 0); assert.equal(accepted, true);
  const second = await fx.run(args); assert.equal(second.code, 1);
  assert.equal(envelopes(second).error.subtype, "creation_request_expired");
  assert.equal(envelopes(second).error.operation_id, "expired-request-001");
  assert.equal(fx.calls.filter((call) => call.method === "POST").length, 1);
});

test("influencer process: signed owner artifact and redirect use no account credentials", async (t) => {
  const bytes = Buffer.from("\x89PNG\r\n\x1a\nredirected-media", "binary");
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url === "/api/influencer/v1/tasks/ready-task/artifacts") {
      assert.equal(call.headers.authorization, "Bearer test-access-token");
      json(res, 200, artifacts(`${fx.baseUrl}/v1/tasks/ready-task/artifacts/image-0/content?access=signed-owner-secret`));
    } else if (call.url.startsWith("/v1/tasks/")) {
      assert.equal(call.headers.authorization, undefined); assert.equal(call.headers["new-api-user"], undefined);
      res.writeHead(302, { location: "https://assets.example.test/final.png?signed=storage-private" }); res.end();
    } else if (call.url.startsWith("/storage/")) {
      assert.equal(call.headers.authorization, undefined); assert.equal(call.headers["new-api-user"], undefined);
      res.writeHead(200, { "content-type": "image/png" }); res.end(bytes);
    } else json(res, 500, { code: "unexpected_network" });
  });
  const output = path.join(fx.root, "redirected.png");
  const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output], {}, { preload: assetPreload(fx) });
  assert.equal(result.code, 0, result.stderr); assert.deepEqual(fs.readFileSync(output), bytes);
  assert.equal(result.stdout.includes("signed-owner-secret"), false); assert.equal(result.stdout.includes("storage-private"), false);
});

test("influencer process: unsigned owner artifact and unsafe HTTPS downgrade are rejected", async (t) => {
  let scenario = "unsigned";
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url.endsWith("/artifacts")) json(res, 200, artifacts(scenario === "unsigned" ? `${fx.baseUrl}/v1/tasks/ready-task/artifacts/image-0/content` : "https://assets.example.test/redirect.png"));
    else if (call.url.startsWith("/storage/")) { res.writeHead(302, { location: `${fx.baseUrl}/forbidden-plain-http` }); res.end(); }
    else { assert.fail("unsafe download URL must never be fetched"); }
  });
  const preload = assetPreload(fx);
  for (scenario of ["unsigned", "downgrade"]) {
    const output = path.join(fx.root, `${scenario}.png`);
    const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output], {}, { preload });
    assert.equal(result.code, 1, result.stderr); assert.equal(envelopes(result).error.subtype, "artifact_unavailable");
    assert.equal(fs.existsSync(output), false);
  }
  assert.equal(fx.calls.some((call) => call.url === "/forbidden-plain-http"), false);
});

test("influencer process: a destination created during streaming is preserved, overwrite requires explicit flag", async (t) => {
  const bytes = Buffer.from("\x89PNG\r\n\x1a\nreplacement-media", "binary");
  let race = true;
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url.endsWith("/artifacts")) json(res, 200, artifacts("https://assets.example.test/race.png"));
    else if (call.url.startsWith("/storage/")) {
      if (race) fs.writeFileSync(path.join(fx.root, "race.png"), "created-during-transfer");
      res.writeHead(200, { "content-type": "image/png", "content-length": bytes.length }); res.end(bytes);
    } else json(res, 500, { code: "unexpected_network" });
  });
  const output = path.join(fx.root, "race.png");
  const args = ["influencer", "task", "download", "ready-task", "--output", output];
  const first = await fx.run(args, {}, { preload: assetPreload(fx) });
  assert.notEqual(first.code, 0); assert.equal(envelopes(first).error.subtype, "output_exists");
  assert.equal(fs.readFileSync(output, "utf8"), "created-during-transfer");
  assert.deepEqual(fs.readdirSync(fx.root).filter((name) => name.endsWith(".part")), []);
  race = false;
  const second = await fx.run([...args, "--overwrite"], {}, { preload: assetPreload(fx) });
  assert.equal(second.code, 0, second.stderr); assert.deepEqual(fs.readFileSync(output), bytes);
});

test("influencer process: total wait timeout also bounds an unresponsive HTTP lookup", async (t) => {
  const fx = await fixture(t, (_req, _res, call) => { assert.equal(call.method, "GET"); });
  const started = Date.now();
  const result = await fx.run(["influencer", "task", "wait", "unresponsive-task", "--timeout", "1"], {}, { timeout: 5000 });
  assert.equal(result.code, 1, result.stderr);
  assert.equal(envelopes(result).error.subtype, "wait_timeout");
  assert.equal(envelopes(result).error.task_id, "unresponsive-task");
  assert.ok(Date.now() - started < 4000, "the HTTP call must not outlive the requested overall wait limit");
});

signalTest("influencer process: interrupted wait exits with resumable task identity", async (t) => {
  let child, lookups = 0;
  const fx = await fixture(t, (_req, res, call) => {
    assert.equal(call.method, "GET");
    if (++lookups === 1) json(res, 200, task({ task_id: "signal-wait-task", status: "running" }));
    else child.kill("SIGINT"); // A second lookup proves the first task identity was parsed.
  });
  const result = await fx.run(["influencer", "task", "wait", "signal-wait-task"], {}, { onSpawn: (value) => { child = value; } });
  assert.equal(result.code, 1, result.stderr);
  const output = envelopes(result);
  assert.equal(output.error.subtype, "wait_interrupted");
  assert.equal(output.error.task_id, "signal-wait-task");
  assert.equal(output.error.operation_id, "request-test-001");
  assert.deepEqual(output.error.recovery.args, ["influencer", "task", "wait", "signal-wait-task"]);
  assert.equal(fx.calls.filter((call) => call.method === "POST").length, 0);
});

signalTest("influencer process: interrupted download removes its own partial and retains original task", async (t) => {
  let child, partObserved = false;
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url.endsWith("/artifacts")) json(res, 200, artifacts("https://assets.example.test/slow.png?access=private-storage"));
    else if (call.url.startsWith("/storage/")) {
      res.writeHead(200, { "content-type": "image/png", "content-length": 1000000 });
      res.write(Buffer.from("\x89PNG\r\n\x1a\npartial-body", "binary"));
      const started = Date.now();
      const probe = setInterval(() => {
        partObserved = fs.readdirSync(fx.root).some((name) => name.endsWith(".part"));
        if (partObserved || Date.now() - started > 3000) { clearInterval(probe); child.kill("SIGINT"); }
      }, 10);
    } else json(res, 500, { code: "unexpected_network" });
  });
  const output = path.join(fx.root, "interrupted-by-user.png");
  const result = await fx.run(["influencer", "task", "download", "ready-task", "--output", output], {}, { preload: assetPreload(fx), onSpawn: (value) => { child = value; } });
  assert.equal(partObserved, true, "the interruption must happen after actual partial creation");
  assert.notEqual(result.code, 0);
  assert.equal(envelopes(result).error.task_id, "ready-task");
  assert.deepEqual(envelopes(result).error.recovery.args, ["influencer", "task", "download", "ready-task", "--output", output]);
  assert.equal(fs.existsSync(output), false);
  assert.deepEqual(fs.readdirSync(fx.root).filter((name) => name.endsWith(".part")), []);
  assert.equal(result.stderr.includes("private-storage"), false);
});

test("influencer process: ordinary nested outputs and recovery never disclose artifact capabilities", async (t) => {
  const capability = "synthetic-private-media-capability-9274";
  const publicMedia = "https://files.example.test/options/public-hair.webp";
  let fx;
  fx = await fixture(t, (_req, res, call) => {
    const content = `${fx.baseUrl}/v1/tasks/output-task-001/artifacts/image/content?access=${capability}`;
    const item = { key: "image", type: "image", mime_type: "image/png", content_url: content };
    const result = task({ request_id: "output-private-001", task_id: "output-task-001", status: "succeeded", artifact_status: "ready", artifacts: [item], multiviews: [{ task_id: "views-001", artifacts: [item] }], nested: { legacy_content_url: content, note: `private artifact ${content}`, data: { artifacts: [item] } } });
    if (call.url.endsWith("/options")) json(res, 200, { taxonomy: { options: [{ imageUrl: publicMedia }] } });
    else if (call.url.includes("/requests/")) json(res, 200, result);
    else if (call.method === "POST") json(res, 200, result);
    else if (call.url.includes("?")) json(res, 200, { items: [result] });
    else json(res, 200, result);
  });
  const commands = [
    ["task", "list"], ["task", "get", "output-task-001"], ["task", "wait", "output-task-001"],
    ["character", "list"], ["character", "get", "output-task-001"], ["request", "get", "output-private-001"],
    ["character", "create", "--brief", "a character", "--request-id", "output-private-001", "--yes"],
    ["character", "create", "--brief", "a character", "--request-id", "output-private-001", "--yes"],
  ];
  for (const command of commands) {
    const result = await fx.run(["influencer", ...command]);
    assert.equal(result.code, 0, result.stderr);
    assert.equal(result.stdout.includes(capability), false, command.join(" "));
    assert.equal(result.stdout.includes("?access="), false, command.join(" "));
    assert.equal(result.stderr.includes(capability), false);
    const data = envelopes(result).data;
    const projected = data.items ? data.items[0] : data;
    assert.deepEqual(projected.artifacts, [{ key: "image", type: "image", mime_type: "image/png" }]);
    assert.deepEqual(projected.multiviews[0].artifacts, projected.artifacts);
    assert.deepEqual(projected.nested.data.artifacts, projected.artifacts);
  }
  const options = await fx.run(["influencer", "options"]);
  assert.equal(options.code, 0, options.stderr);
  assert.equal(envelopes(options).data.taxonomy.options[0].imageUrl, publicMedia);
  for (const filename of walk(fx.config)) assert.equal(fs.readFileSync(filename, "utf8").includes(capability), false, filename);
});

test("influencer process: early confirmation and API error text redact media capability URLs", async (t) => {
  const capability = "synthetic-error-media-capability-4285";
  let signed;
  const fx = await fixture(t, (_req, res) => json(res, 503, { code: "artifact_projection_unavailable", message: `cannot read ${signed}` }));
  signed = `${fx.baseUrl}/v1/tasks/error-task/artifacts/image/content?access=${capability}`;
  const confirmed = await fx.run(["influencer", "character", "create", "--reference-image", signed]);
  assert.equal(confirmed.code, 10, confirmed.stderr);
  assert.equal(confirmed.stderr.includes(capability), false);
  assert.equal(fx.calls.length, 0);
  const failed = await fx.run(["influencer", "task", "get", "error-task"]);
  assert.equal(failed.code, 1, failed.stderr);
  assert.equal(envelopes(failed).error.subtype, "artifact_projection_unavailable");
  assert.equal(failed.stderr.includes(capability), false);
  assert.equal(failed.stdout, "");
  assert.deepEqual(walk(fx.config), []);
});

test("influencer process: partial HTTP media can never become a completed download", async (t) => {
  let status = 206, contentRange = "bytes 0-15/10000";
  const bytes = Buffer.from([0, 0, 0, 16, 102, 116, 121, 112, 105, 115, 111, 109, 0, 0, 0, 0]);
  const fx = await fixture(t, (_req, res, call) => {
    if (call.url.endsWith("/artifacts")) json(res, 200, { task_id: "partial-task", artifacts: [{ key: "video", type: "video", mime_type: "video/mp4", content_url: "https://assets.example.test/partial.mp4?signature=synthetic-partial-capability" }] });
    else if (call.url.startsWith("/storage/")) {
      assert.equal(call.headers.authorization, undefined);
      assert.equal(call.headers.range, undefined);
      res.writeHead(status, { "content-type": "video/mp4", "content-length": bytes.length, ...(contentRange ? { "content-range": contentRange } : {}) });
      res.end(bytes);
    } else json(res, 500, { code: "unexpected_network" });
  });
  const preload = assetPreload(fx);
  for (const [code, range] of [[206, "bytes 0-15/10000"], [206, ""], [200, "bytes 0-15/10000"], [201, ""]]) {
    status = code; contentRange = range;
    const output = path.join(fx.root, `partial-${code}-${range ? "range" : "no-range"}.mp4`);
    const result = await fx.run(["influencer", "task", "download", "partial-task", "--output", output], {}, { preload });
    assert.equal(result.code, 1, `${code} ${range}: ${result.stderr}`);
    assert.equal(envelopes(result).error.subtype, "artifact_invalid");
    assert.equal(fs.existsSync(output), false);
    assert.equal(result.stderr.includes("synthetic-partial-capability"), false);
  }
  assert.deepEqual(fs.readdirSync(fx.root).filter((name) => name.endsWith(".part")), []);
});
