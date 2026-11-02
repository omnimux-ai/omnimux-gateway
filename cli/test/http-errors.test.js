const { test } = require("node:test");
const assert = require("node:assert/strict");

const {
  classifyHttpFailure,
  classifyTransportFailure,
} = require("../src/lib/http");

test("HTTP 401 maps to auth/token_invalid", () => {
  const err = classifyHttpFailure(401, { message: "unauthorized" });
  assert.equal(err.type, "auth");
  assert.equal(err.subtype, "token_invalid");
  assert.equal(err.exitCode, 2);
});

test("HTTP 429 maps to api/rate_limit retryable", () => {
  const err = classifyHttpFailure(429, { message: "slow down" });
  assert.equal(err.type, "api");
  assert.equal(err.subtype, "rate_limit");
  assert.equal(err.retryable, true);
});

test("HTTP 404 maps to api/not_found", () => {
  const err = classifyHttpFailure(404, {});
  assert.equal(err.subtype, "not_found");
});

test("HTTP 500 maps to api/upstream retryable", () => {
  const err = classifyHttpFailure(500, { message: "boom" });
  assert.equal(err.subtype, "upstream");
  assert.equal(err.retryable, true);
});

test("unavailable creation routes preserve stable non-retryable reasons and original identities", () => {
  for (const code of ["creation_route_unavailable", "generation_route_unavailable", "reference_route_unavailable"]) {
    const err = classifyHttpFailure(503, {
      code,
      message: "creation route unavailable",
      operation_id: "character-request-001",
      task_id: "original-task-001",
    });
    assert.equal(err.type, "api");
    assert.equal(err.subtype, code);
    assert.equal(err.retryable, false);
    assert.equal(err.operationId, "character-request-001");
    assert.equal(err.taskId, "original-task-001");
    assert.equal(err.exitCode, 1);
  }
});

test("AbortError maps to network/timeout", () => {
  const abort = new Error("aborted");
  abort.name = "AbortError";
  const err = classifyTransportFailure(abort, "/api/x");
  assert.equal(err.type, "network");
  assert.equal(err.subtype, "timeout");
  assert.equal(err.retryable, true);
  assert.equal(err.exitCode, 4);
});

test("generic fetch failure maps to network/transport", () => {
  const err = classifyTransportFailure(new Error("ECONNREFUSED"), "/api/x");
  assert.equal(err.type, "network");
  assert.equal(err.subtype, "transport");
});
