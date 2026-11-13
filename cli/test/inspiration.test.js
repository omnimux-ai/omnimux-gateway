const { test } = require("node:test");
const assert = require("node:assert/strict");

const { parseInspirationFlags, run } = require("../src/commands/inspiration");

test("parseInspirationFlags: parses category, title, prompt, media", () => {
  const { flags, positional } = parseInspirationFlags([
    "publish",
    "--category",
    "电影街景",
    "--title",
    "90年代曼哈顿",
    "--prompt",
    '{"format":"16:9"}',
    "--model",
    "Seedance 2.5",
    "--media",
    "https://example.com/video.mp4",
    "--cover",
    "https://example.com/cover.jpg",
  ]);

  assert.deepEqual(positional, ["publish"]);
  assert.equal(flags.category, "电影街景");
  assert.equal(flags.title, "90年代曼哈顿");
  assert.equal(flags.prompt, '{"format":"16:9"}');
  assert.equal(flags.model, "Seedance 2.5");
  assert.equal(flags.mediaUrl, "https://example.com/video.mp4");
  assert.equal(flags.coverUrl, "https://example.com/cover.jpg");
});

test("inspiration publish: throws UsageError when category is missing", async () => {
  await assert.rejects(
    async () => {
      await run(
        ["publish", "--title", "Title", "--prompt", "Prompt"],
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        {}
      );
    },
    {
      name: "UsageError",
      message: /必须包含灵感分类才能发布/,
    }
  );
});

test("inspiration publish: throws UsageError when title or prompt is missing", async () => {
  await assert.rejects(
    async () => {
      await run(
        ["publish", "--category", "街景", "--prompt", "Prompt"],
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        {}
      );
    },
    {
      name: "UsageError",
      message: /必须包含灵感核心数据/,
    }
  );

  await assert.rejects(
    async () => {
      await run(
        ["publish", "--category", "街景", "--title", "Title"],
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        {}
      );
    },
    {
      name: "UsageError",
      message: /必须包含灵感核心数据/,
    }
  );
});

test("inspiration get: throws UsageError when share_id is missing", async () => {
  await assert.rejects(
    async () => {
      await run(
        ["get"],
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        {}
      );
    },
    {
      name: "UsageError",
      message: /usage: omnimux inspiration get <share_id>/,
    }
  );
});

test("parseInspirationFlags: parses share id and expire", () => {
  const { flags, positional } = parseInspirationFlags([
    "share",
    "2789",
    "--expire",
    "3days",
  ]);

  assert.deepEqual(positional, ["share", "2789"]);
  assert.equal(flags.expire, "3days");
});

test("parseInspirationFlags: accepts --id and --inspiration-id as the id spelling", () => {
  assert.equal(parseInspirationFlags(["share", "--id", "2789"]).flags.id, "2789");
  assert.equal(
    parseInspirationFlags(["share", "--inspiration-id=2789"]).flags.id,
    "2789"
  );
});

test("inspiration share: throws UsageError when the inspiration id is missing", async () => {
  await assert.rejects(
    async () => {
      await run(
        ["share"],
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        {}
      );
    },
    {
      name: "UsageError",
      message: /usage: omnimux inspiration share <inspiration_id>/,
    }
  );
});

test("uploadGatewayFile: throws UsageError when file does not exist", async () => {
  const { uploadGatewayFile } = require("../src/lib/upload");
  await assert.rejects(
    async () => {
      await uploadGatewayFile(
        { baseUrl: "http://127.0.0.1", accessToken: "test" },
        "/path/to/nonexistent/media-file.mp4"
      );
    },
    {
      name: "UsageError",
      message: /file not found/,
    }
  );
});

test("inspiration publish: uploads local file automatically when path exists", async () => {
  const os = require("os");
  const fs = require("fs");
  const path = require("path");

  // Create temporary local file
  const tmpFile = path.join(os.tmpdir(), `test-local-media-${Date.now()}.png`);
  fs.writeFileSync(tmpFile, Buffer.from("fake-image-bytes"));

  let uploadedFile = false;
  let publishPayload = null;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, opts) => {
    const urlStr = String(url);
    if (urlStr.includes("/api/v1/files/upload/stream")) {
      uploadedFile = true;
      return {
        ok: true,
        status: 200,
        headers: new Headers({ "content-type": "application/json" }),
        text: async () =>
          JSON.stringify({
            success: true,
            code: 200,
            data: {
              file_id: "file_mock_123456",
              file_url: "https://omnimux.ai/api/v1/files/download/file_mock_123456",
            },
          }),
      };
    }
    if (urlStr.includes("/api/inspiration/v1/share")) {
      publishPayload = JSON.parse(opts.body);
      return {
        ok: true,
        status: 200,
        headers: new Headers({ "content-type": "application/json" }),
        text: async () =>
          JSON.stringify({
            success: true,
            data: {
              share_id: "insp_mock_789",
              share_url: "https://omnimux.ai/s/insp_mock_789",
            },
          }),
      };
    }
    return originalFetch(url, opts);
  };

  try {
    await run(
      [
        "publish",
        "--category",
        "带货短剧",
        "--title",
        "本地图片测试",
        "--prompt",
        "提示词内容",
        "--media",
        tmpFile,
      ],
      { baseUrl: "http://127.0.0.1", accessToken: "test-token" },
      {}
    );

    assert.equal(uploadedFile, true, "Local media file must be uploaded to gateway file service first");
    assert.ok(publishPayload, "Share payload must be submitted");
    assert.equal(
      publishPayload.media_url,
      "https://omnimux.ai/api/v1/files/download/file_mock_123456",
      "Media URL must be replaced with the R2 download URL"
    );
    assert.equal(publishPayload.media_type, "image", "Media type should default to image for .png");
  } finally {
    globalThis.fetch = originalFetch;
    try {
      fs.unlinkSync(tmpFile);
    } catch {}
  }
});

test("parseInspirationFlags: parses group flag for categories", () => {
  assert.equal(parseInspirationFlags(["categories", "--group", "ecommerce"]).flags.group, "ecommerce");
  assert.equal(parseInspirationFlags(["categories", "--group=video"]).flags.group, "video");
});

test("inspiration categories: fetches category dictionary", async () => {
  let requestedUrl = "";
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, opts) => {
    requestedUrl = String(url);
    return {
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      text: async () =>
        JSON.stringify({
          success: true,
          data: {
            total: 2,
            items: [
              { key: "ecommerce-main-image", name: "电商主图" },
              { key: "product-ads", name: "产品广告" },
            ],
          },
        }),
    };
  };

  try {
    await run(["categories", "--group", "ecommerce"], { baseUrl: "http://127.0.0.1" }, {});
    assert.ok(requestedUrl.includes("/api/inspiration/v1/categories?group=ecommerce"));
  } finally {
    globalThis.fetch = originalFetch;
  }
});
