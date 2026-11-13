const { test } = require("node:test");
const assert = require("node:assert/strict");

const {
  sanitize,
  maskTokenKey,
  maskSensitiveJson,
  isSensitivePath,
} = require("../src/lib/mask");

test("sanitize redacts sk- tokens", () => {
  assert.equal(sanitize("key is sk-abcdef1234567890 ok"), "key is sk-<REDACTED> ok");
});

test("sanitize redacts Bearer tokens", () => {
  // Rule 2 masks the token; rule 4 then masks the whole sensitive-named
  // ("Authorization") line value — ported behavior from sanitize.js.
  assert.equal(sanitize("Authorization: Bearer abcdef123456"), "Authorization: <REDACTED>");
  assert.equal(sanitize('header = "Bearer abcdef123456"'), 'header = "Bearer <REDACTED>"');
});

test("sanitize redacts credential connection strings", () => {
  assert.equal(sanitize("postgres://user:pass@db.host:5432/x"), "postgres://<REDACTED>");
});

test("sanitize redacts JSON sensitive field values", () => {
  assert.equal(
    sanitize('  "api_key": "supersecretvalue",'),
    '  "api_key": "<REDACTED>",'
  );
});

test("sanitize redacts YAML and ENV sensitive values", () => {
  assert.equal(sanitize("password: hunter2"), "password: <REDACTED>");
  assert.equal(sanitize("OPENAI_API_KEY=abcdef"), "OPENAI_API_KEY=<REDACTED>");
});

test("sanitize keeps non-sensitive lines untouched", () => {
  assert.equal(sanitize('  "model": "gpt-4o",'), '  "model": "gpt-4o",');
});

test("maskTokenKey masks long keys and keeps sk- prefix", () => {
  assert.equal(maskTokenKey("sk-abcdefghijklmnop"), "sk-abcd**********mnop");
});

test("maskTokenKey is idempotent over server-masked keys", () => {
  const serverMasked = "reHR**********OspA";
  assert.equal(maskTokenKey(serverMasked), serverMasked);
});

test("maskTokenKey handles short and empty values", () => {
  assert.equal(maskTokenKey("abcd"), "****");
  assert.equal(maskTokenKey("abcdef12"), "ab****12");
  assert.equal(maskTokenKey("sk-"), "sk-");
  assert.equal(maskTokenKey(42), 42);
});

test("maskSensitiveJson masks sensitive keys deeply", () => {
  const input = {
    success: true,
    data: {
      items: [
        { id: 1, key: "sk-abcdefghijklmnop", name: "prod" },
        { id: 2, keys: ["abcdefghijklmnop", 7], name: "backup" },
      ],
      access_token: "tok_abcdefghijklmnopqrst",
    },
  };
  const masked = maskSensitiveJson(input);
  assert.equal(masked.data.items[0].key, "sk-abcd**********mnop");
  assert.equal(masked.data.items[0].name, "prod");
  assert.equal(masked.data.items[1].keys[0], "abcd**********mnop");
  assert.equal(masked.data.items[1].keys[1], 7);
  assert.equal(masked.data.access_token, "tok_**********qrst");
});

test("isSensitivePath matches key-carrying user paths only", () => {
  assert.equal(isSensitivePath("/api/token/"), true);
  assert.equal(isSensitivePath("/api/user/self"), true);
  assert.equal(isSensitivePath("/api/pricing"), false);
  assert.equal(isSensitivePath("/api/user/models"), true);
});

test("sanitize removes media capabilities while preserving public option media", () => {
  const publicMedia = "https://files.example.test/options/face.webp";
  const capability = "synthetic-mask-capability-7295";
  for (const url of [
    `https://files.example.test/v1/tasks/task-1/artifacts/image/content?access=${capability}`,
    `/v1/tasks/task-1/artifacts/image/content?access=${capability}`,
    `https://storage.example.test/image.png?X-Amz-Signature=${capability}`,
    `https://storage.example.test/image.png?X-Goog-Signature=${capability}`,
  ]) {
    const value = sanitize(`download failed: ${url}`);
    assert.equal(value.includes(capability), false);
    assert.equal(value.includes("[PRIVATE_MEDIA_URL]"), true);
  }
  assert.equal(sanitize(publicMedia), publicMedia);
  assert.equal(sanitize("high-risk-write"), "high-risk-write");
  assert.equal(sanitize("sk-genuinecredential1234"), "sk-<REDACTED>");
});
