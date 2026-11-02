/** Owner-scoped creation workflow. Commands only adapt argv/output to this library. */
const crypto = require("crypto");
const { ApiError, asCliError } = require("./errors");
const { uploadGatewayFile } = require("./upload");
const journal = require("./influencer-receipts");
const { API_ROOT, LIMITS, createInput, materials, isRemote, validateLocalFile, canonical } = require("./influencer-input");
function unwrap(response) {
  if (!response || response.success !== true || response.data === undefined) throw new ApiError("creation service returned an invalid response", { subtype: "invalid_response" });
  return response.data;
}
async function request(ctx, method, suffix, body, options) {
  try { return unwrap(await ctx.api(method, API_ROOT + suffix, body, undefined, { ...options, noRetry: method === "POST" })); }
  catch (err) {
    if (suffix === "/options" && err.status === 404) throw new ApiError("the configured service does not support character/video CLI creation", { subtype: "creation_not_supported", status: 404 });
    throw err;
  }
}
function recovery(id) { return { command: "omnimux", args: ["influencer", "request", "get", id] }; }
function resumable(err, id, taskId) {
  err = asCliError(err);
  err.operationId = id;
  if (taskId) err.taskId = taskId;
  err.recovery = recovery(id);
  return err;
}
async function restore(ctx, id) {
  const restored = await request(ctx, "GET", `/requests/${encodeURIComponent(id)}`);
  if (!restored || restored.request_id !== id || typeof restored.task_id !== "string" || !restored.task_id) throw new ApiError("request lookup omitted its original recovery identity", { subtype: "invalid_response" });
  return restored;
}
async function create(ctx, kind, flags, parentId) {
  const body = createInput(kind, flags);
  const refs = materials(kind, body);
  if (ctx.dryRun) {
    for (const ref of refs) if (!isRemote(ref.value)) validateLocalFile(ref.value, ref.kind);
    return { dry_run: true, kind, ...(parentId ? { parent_task_id: parentId } : {}), input: body, local_validation: "passed", server_validation: "pending", media_inspection: "pending", submitted: false };
  }
  const id = body.request_id || crypto.randomUUID();
  body.request_id = id;
  const intent = canonical({ kind, ...(parentId ? { parent_task_id: parentId } : {}), input: body });
  let record;
  try { record = await journal.prepare(ctx.auth, id, intent, refs); }
  catch (err) { throw resumable(err, id); }
  const receipt = record.receipt;
  try {
    if (receipt.attempted) {
      let restored;
      try {
        restored = await restore(ctx, id);
      } catch (err) {
        if (err.type !== "api" || err.status !== 404 || err.subtype !== "not_found") throw err;
        // A definitive 404 permits only this exact persisted body/key. Never upload again.
        if (!receipt.body) throw new ApiError("creation outcome requires reconciliation", { subtype: "creation_outcome_unknown" });
      }
      if (restored) {
        receipt.stage = "accepted"; receipt.task_id = restored.task_id; record.save();
        return restored;
      }
    } else {
      for (const ref of refs) {
        if (isRemote(ref.value)) continue;
        const key = journal.materialKey(ref);
        if (receipt.uploads[key]) continue;
        const result = await uploadGatewayFile(ctx.auth, ref.value, ref.kind === "video" ? "videos" : "photos", { maxBytes: LIMITS[ref.kind], expectedSha256: receipt.fingerprints[key] });
        const url = result && result.file_url;
        if (typeof url !== "string" || !url.startsWith("https://")) throw new ApiError("media upload did not return a usable HTTPS reference", { subtype: "invalid_response" });
        const parsed = new URL(url);
        if (parsed.username || parsed.password) throw new ApiError("media upload returned an unsafe reference", { subtype: "invalid_response" });
        receipt.uploads[key] = url; receipt.stage = "uploaded"; record.save();
      }
      const finalBody = JSON.parse(JSON.stringify(body));
      for (const ref of refs) {
        if (isRemote(ref.value)) continue;
        const url = receipt.uploads[journal.materialKey(ref)];
        if (ref.index === undefined) finalBody[ref.field] = url;
        else finalBody[ref.field][ref.index] = url;
      }
      receipt.body = finalBody; receipt.stage = "ready"; record.save();
    }
    // Mark before fetch: a killed process must query the request receipt first.
    receipt.attempted = true; receipt.stage = "submitting"; record.save();
    const suffix = kind === "character" ? "/characters" : kind === "video" ? "/videos" : `/characters/${encodeURIComponent(parentId)}/multiview`;
    let result;
    try { result = await request(ctx, "POST", suffix, receipt.body); }
    catch (err) {
      // Never retry the POST here. A transport failure can mean the cloud accepted it.
      receipt.stage = "reconcile";
      try { record.save(); } catch {}
      throw err;
    }
    if (!result || result.request_id !== id || !result.task_id) throw new ApiError("creation response omitted its recovery identity", { subtype: "invalid_response" });
    receipt.task_id = result.task_id; receipt.stage = "accepted"; record.save();
    return result;
  } catch (err) { throw resumable(err, id, receipt.task_id); }
  finally { record.release(); }
}
async function wait(ctx, taskId, seconds = 600) {
  const deadline = Date.now() + seconds * 1000;
  const controller = new AbortController();
  let interrupted = false, lastTask;
  const interrupt = () => { interrupted = true; controller.abort(); };
  process.once("SIGINT", interrupt); process.once("SIGTERM", interrupt);
  const recovery = { command: "omnimux", args: ["influencer", "task", "wait", taskId] };
  function stopped(subtype, message) {
    return new ApiError(message, { subtype, operationId: lastTask?.request_id, taskId, retryable: true, recovery });
  }
  try {
    for (;;) {
      if (interrupted) throw stopped("wait_interrupted", "waiting was interrupted; the cloud task continues");
      const left = deadline - Date.now();
      if (left <= 0) throw stopped("wait_timeout", "waiting timed out; the cloud task continues");
      try {
        lastTask = await request(ctx, "GET", `/tasks/${encodeURIComponent(taskId)}`, undefined, { timeoutMs: Math.min(20000, left), signal: controller.signal });
      } catch (err) {
        if (interrupted) throw stopped("wait_interrupted", "waiting was interrupted; the cloud task continues");
        if (Date.now() >= deadline) throw stopped("wait_timeout", "waiting timed out; the cloud task continues");
        err.taskId = taskId; err.operationId ||= lastTask?.request_id; err.recovery ||= recovery;
        throw err;
      }
      if (lastTask.status === "succeeded") {
        if (lastTask.artifact_status !== "ready") throw new ApiError("generation completed without a usable artifact", { subtype: "artifact_unavailable", operationId: lastTask.request_id, taskId, recovery });
        return lastTask;
      }
      if (lastTask.status === "failed") throw new ApiError("generation failed; inspect the original task instead of resubmitting", { subtype: "generation_failed", operationId: lastTask.request_id, taskId, recovery: { command: "omnimux", args: ["influencer", "task", "get", taskId] } });
      await new Promise((resolve) => {
        const timer = setTimeout(done, Math.min(2000, Math.max(1, deadline - Date.now())));
        function done() { clearTimeout(timer); controller.signal.removeEventListener("abort", done); resolve(); }
        controller.signal.addEventListener("abort", done, { once: true });
        if (controller.signal.aborted) done();
      });
    }
  } finally {
    process.removeListener("SIGINT", interrupt); process.removeListener("SIGTERM", interrupt);
  }
}

module.exports = { request, create, restore, wait, unwrap };
