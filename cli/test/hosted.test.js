const { test } = require("node:test");
const assert = require("node:assert/strict");

const hosted = require("../src/commands/hosted");
const { UsageError } = require("../src/lib/output");

function mockCtx(args, apiImpl, flags = {}) {
  const outputs = [];
  return {
    args,
    flags,
    dryRun: Boolean(flags.dryRun),
    out: {
      ok: (data) => outputs.push(data),
      fail: (err) => {},
    },
    outputs,
    api: apiImpl || (async () => ({ success: true, data: {} })),
  };
}

test("hosted status returns payload from GET /api/hosted/v1/status", async () => {
  const ctx = mockCtx(["status"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/status");
    return {
      success: true,
      data: {
        enabled: true,
        gateway_ready: true,
        billing_mode: "wallet",
        sku_platform: "tiktok",
        sku_region: "us",
        price_usd: 99,
      },
    };
  });

  await hosted.run(ctx);
  assert.equal(ctx.outputs.length, 1);
  const data = ctx.outputs[0];
  assert.equal(data.gateway_ready, true);
  assert.equal(data.price_usd, 99);
});

test("hosted skus returns SKU array from GET /api/hosted/v1/skus", async () => {
  const ctx = mockCtx(["skus"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/skus");
    return {
      success: true,
      data: [
        { id: 1, platform: "tiktok", region: "us", price_usd: 99, stock: 2, est_delivery_hours: 0 },
      ],
    };
  });

  await hosted.run(ctx);
  const data = ctx.outputs[0];
  assert.equal(data.skus.length, 1);
  assert.equal(data.skus[0].platform, "tiktok");
});

test("hosted channels returns channels array and supports status query", async () => {
  const ctx = mockCtx(["channels", "--status=ready"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/channels?status=ready");
    return {
      success: true,
      data: [
        { id: 1, platform: "tiktok", region: "us", status: "ready", readiness: 100 },
      ],
    };
  });

  await hosted.run(ctx);
  const data = ctx.outputs[0];
  assert.equal(data.channels.length, 1);
  assert.equal(data.channels[0].status, "ready");
});

test("hosted channel detail requires channel id", async () => {
  const ctx = mockCtx(["channel"]);
  await assert.rejects(async () => hosted.run(ctx), UsageError);
});

test("hosted brand sends PATCH to /api/hosted/v1/channels/:id/brand", async () => {
  let patchedBody = null;
  const ctx = mockCtx(
    ["brand", "1", "--bio=Official Account", "--site=https://brand.com", "--keywords=test1,test2"],
    async (method, path, body) => {
      assert.equal(method, "PATCH");
      assert.equal(path, "/api/hosted/v1/channels/1/brand");
      patchedBody = body;
      return { success: true, data: { ok: true } };
    }
  );

  await hosted.run(ctx);
  assert.deepEqual(patchedBody, {
    bio: "Official Account",
    site: "https://brand.com",
    keywords: ["test1", "test2"],
  });
});

test("hosted post validates channel and media/caption presence", async () => {
  const ctx = mockCtx(["post", "--channel=1"]);
  await assert.rejects(async () => hosted.run(ctx), /either --media\/--file or --caption is required/);
});

test("hosted post submits task to /api/hosted/v1/channels/:id/tasks", async () => {
  let submittedBody = null;
  const ctx = mockCtx(
    ["post", "--channel=2", "--media=https://cdn.com/v.mp4", "--caption=Promo video", "--idempotency-key=post-1"],
    async (method, path, body) => {
      assert.equal(method, "POST");
      assert.equal(path, "/api/hosted/v1/channels/2/tasks");
      submittedBody = body;
      return { success: true, data: { task_id: 101, status: "pending" } };
    }
  );

  await hosted.run(ctx);
  assert.equal(submittedBody.type, "distribute");
  assert.equal(submittedBody.media_url, "https://cdn.com/v.mp4");
  assert.equal(submittedBody.caption, "Promo video");
  assert.equal(submittedBody.idempotency_key, "post-1");
  const data = ctx.outputs[0];
  assert.equal(data.task_id, 101);
});

test("hosted batch submits multiple channel IDs", async () => {
  let batchBody = null;
  const ctx = mockCtx(
    ["batch", "--channels=1,2,3", "--media=https://cdn.com/v.mp4", "--caption=Matrix"],
    async (method, path, body) => {
      assert.equal(method, "POST");
      assert.equal(path, "/api/hosted/v1/tasks/batch");
      batchBody = body;
      return {
        success: true,
        data: {
          total: 3,
          submitted: 3,
          tasks: [{ task_id: 10, channel_id: 1 }],
        },
      };
    }
  );

  await hosted.run(ctx);
  assert.deepEqual(batchBody.channel_ids, [1, 2, 3]);
  assert.equal(batchBody.caption, "Matrix");
});

test("hosted cancel submits POST /api/hosted/v1/tasks/:id/cancel", async () => {
  let cancelledPath = "";
  const ctx = mockCtx(["cancel", "55"], async (method, path) => {
    assert.equal(method, "POST");
    cancelledPath = path;
    return { success: true, data: { ok: true } };
  });

  await hosted.run(ctx);
  assert.equal(cancelledPath, "/api/hosted/v1/tasks/55/cancel");
  const data = ctx.outputs[0];
  assert.equal(data.task_id, 55);
});

test("hosted buy --dry-run previews order without calling API", async () => {
  let called = false;
  const ctx = mockCtx(
    ["buy", "--sku=1", "--quantity=2", "--bio=Brand Bio"],
    async () => {
      called = true;
      return { success: true };
    },
    { dryRun: true }
  );

  await hosted.run(ctx);
  assert.equal(called, false);
  const data = ctx.outputs[0];
  assert.equal(data.dry_run, true);
  assert.equal(data.quantity, 2);
  assert.equal(data.estimated_amount_usd, 198);
});

test("hosted defaults to status when no subcommand is provided", async () => {
  let calledPath = "";
  const ctx = mockCtx([], async (method, path) => {
    calledPath = path;
    return { success: true, data: { enabled: true } };
  });
  await hosted.run(ctx);
  assert.equal(calledPath, "/api/hosted/v1/status");
  assert.equal(ctx.outputs[0].enabled, true);
});

test("hosted channel gets details by positional id and by --channel flag", async () => {
  const ctx1 = mockCtx(["channel", "42"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/channels/42");
    return { success: true, data: { id: 42, name: "Channel 42" } };
  });
  await hosted.run(ctx1);
  assert.equal(ctx1.outputs[0].id, 42);

  const ctx2 = mockCtx(["channel", "--channel=99"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/channels/99");
    return { success: true, data: { id: 99, name: "Channel 99" } };
  });
  await hosted.run(ctx2);
  assert.equal(ctx2.outputs[0].id, 99);
});

test("hosted brand validates required fields and dryRun safety", async () => {
  // missing channel id
  const ctxMissingId = mockCtx(["brand", "--bio=test"]);
  await assert.rejects(async () => hosted.run(ctxMissingId), /channel id is required/);

  // missing all brand fields
  const ctxMissingFields = mockCtx(["brand", "10"]);
  await assert.rejects(async () => hosted.run(ctxMissingFields), /at least one brand field/);

  // dryRun mode does not call API
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["brand", "10", "--nickname=MyNick", "--bio=MyBio", "--site=https://example.com"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].channel_id, 10);
  assert.deepEqual(ctxDry.outputs[0].brand, {
    nick_name: "MyNick",
    bio: "MyBio",
    site: "https://example.com",
  });
});

test("hosted upload validates file path, existence, and dryRun safety", async () => {
  // missing file path
  const ctxMissing = mockCtx(["upload"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /file path is required/);

  // dryRun returns preview without touching filesystem or calling API
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["upload", "./sample-video.mp4"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].file, "./sample-video.mp4");
  assert.equal(ctxDry.outputs[0].content_type, "video/mp4");

  // non-existent file in real mode throws UsageError
  const ctxNonExistent = mockCtx(["upload", "./non-existent-file-xyz.mp4"]);
  await assert.rejects(async () => hosted.run(ctxNonExistent), /file not found/);
});

test("hosted post validates channel ID and handles dryRun with file and media", async () => {
  // missing channel ID
  const ctxMissingChannel = mockCtx(["post", "--caption=Hello"]);
  await assert.rejects(async () => hosted.run(ctxMissingChannel), /channel id is required/);

  // dryRun with media URL
  let apiCalled = false;
  const ctxDryMedia = mockCtx(
    ["post", "--channel=5", "--media=https://cdn.com/test.mp4", "--caption=Test Cap"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDryMedia);
  assert.equal(apiCalled, false);
  assert.equal(ctxDryMedia.outputs[0].dry_run, true);
  assert.equal(ctxDryMedia.outputs[0].channel_id, 5);
  assert.equal(ctxDryMedia.outputs[0].task.media_url, "https://cdn.com/test.mp4");
  assert.equal(ctxDryMedia.outputs[0].task.caption, "Test Cap");

  // dryRun with local file generates dry-run mock URL
  const ctxDryFile = mockCtx(
    ["post", "--channel=5", "--file=my-video.mp4", "--caption=Test File"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDryFile);
  assert.equal(apiCalled, false);
  assert.equal(ctxDryFile.outputs[0].dry_run, true);
  assert.ok(ctxDryFile.outputs[0].task.media_url.includes("dry-run/my-video.mp4"));
});

test("hosted batch validates channel IDs and handles dryRun", async () => {
  // missing channels
  const ctxMissing = mockCtx(["batch", "--caption=Batch Cap"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /at least one channel id is required/);

  // invalid channel id
  const ctxInvalid = mockCtx(["batch", "--channels=1,abc,3", "--caption=Batch Cap"]);
  await assert.rejects(async () => hosted.run(ctxInvalid), /invalid channel id: abc/);

  // missing media and caption
  const ctxNoMediaNoCap = mockCtx(["batch", "--channels=1,2,3"]);
  await assert.rejects(async () => hosted.run(ctxNoMediaNoCap), /either --media\/--file or --caption is required/);

  // dryRun mode
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["batch", "--channels=1,2,3", "--media=https://cdn.com/v.mp4", "--caption=Matrix", "--idempotency-key=key-1"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].channels_count, 3);
  assert.deepEqual(ctxDry.outputs[0].batch.channel_ids, [1, 2, 3]);
  assert.equal(ctxDry.outputs[0].batch.idempotency_key, "key-1");
});

test("hosted tasks gets channel tasks and validates channel ID", async () => {
  // missing channel ID
  const ctxMissing = mockCtx(["tasks"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /channel id is required/);

  // happy path
  const ctx = mockCtx(["tasks", "8"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/channels/8/tasks");
    return { success: true, data: [{ task_id: 1, status: "completed" }] };
  });
  await hosted.run(ctx);
  assert.equal(ctx.outputs[0].channel_id, 8);
  assert.equal(ctx.outputs[0].tasks.length, 1);
});

test("hosted task gets single task details and validates task ID", async () => {
  // missing task ID
  const ctxMissing = mockCtx(["task"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /task id is required/);

  // happy path
  const ctx = mockCtx(["task", "105"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/tasks/105");
    return { success: true, data: { task_id: 105, status: "running", progress: 60 } };
  });
  await hosted.run(ctx);
  assert.equal(ctx.outputs[0].task_id, 105);
  assert.equal(ctx.outputs[0].progress, 60);
});

test("hosted cancel validates task ID and dryRun mode", async () => {
  // missing task ID
  const ctxMissing = mockCtx(["cancel"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /task id is required/);

  // dryRun mode
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["cancel", "99"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].task_id, 99);
});

test("hosted order gets single order details and validates order ID", async () => {
  // missing order ID
  const ctxMissing = mockCtx(["order"]);
  await assert.rejects(async () => hosted.run(ctxMissing), /order id is required/);

  // happy path
  const ctx = mockCtx(["order", "ord-12345"], async (method, path) => {
    assert.equal(method, "GET");
    assert.equal(path, "/api/hosted/v1/orders/ord-12345");
    return { success: true, data: { order_id: "ord-12345", status: "paid", amount_usd: 99 } };
  });
  await hosted.run(ctx);
  assert.equal(ctx.outputs[0].order_id, "ord-12345");
  assert.equal(ctx.outputs[0].status, "paid");
});

test("hosted buy validates quantity and executes real POST call", async () => {
  // invalid quantity: 0, negative, non-integer
  const ctxZero = mockCtx(["buy", "--sku=1", "--quantity=0"]);
  await assert.rejects(async () => hosted.run(ctxZero), /quantity must be a positive integer/);

  const ctxNeg = mockCtx(["buy", "--sku=1", "--quantity=-3"]);
  await assert.rejects(async () => hosted.run(ctxNeg), /quantity must be a positive integer/);

  const ctxFloat = mockCtx(["buy", "--sku=1", "--quantity=1.5"]);
  await assert.rejects(async () => hosted.run(ctxFloat), /quantity must be a positive integer/);

  // real POST call
  let postBody = null;
  const ctxReal = mockCtx(
    ["buy", "--sku=2", "--quantity=3", "--nickname=ShopOfficial", "--site=https://shop.com", "--keywords=kw1,kw2"],
    async (method, path, body) => {
      assert.equal(method, "POST");
      assert.equal(path, "/api/hosted/v1/orders");
      postBody = body;
      return { success: true, data: { order_id: "ord-new-1", status: "created" } };
    }
  );
  await hosted.run(ctxReal);
  assert.equal(postBody.sku_id, 2);
  assert.equal(postBody.quantity, 3);
  assert.equal(postBody.brand.nick_name, "ShopOfficial");
  assert.deepEqual(postBody.brand.keywords, ["kw1", "kw2"]);
  assert.equal(ctxReal.outputs[0].order_id, "ord-new-1");
});

test("hosted addon validates quantity, dryRun, and executes real POST call", async () => {
  // invalid quantity
  const ctxInvalid = mockCtx(["addon", "--sku=1", "--quantity=0"]);
  await assert.rejects(async () => hosted.run(ctxInvalid), /quantity must be a positive integer/);

  // dryRun mode
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["addon", "1", "--quantity=2", "--confirm"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].action, "hosted.addon");
  assert.equal(ctxDry.outputs[0].sku_id, 1);
  assert.equal(ctxDry.outputs[0].quantity, 2);
  assert.equal(ctxDry.outputs[0].confirm, true);

  // real POST call
  let addonPath = "";
  let addonBody = null;
  const ctxReal = mockCtx(
    ["addon", "--sku=5", "--quantity=1", "--confirm", "--idempotency-key=add-1"],
    async (method, path, body) => {
      assert.equal(method, "POST");
      addonPath = path;
      addonBody = body;
      return { success: true, data: { addon_id: "add-rec-1", status: "confirmed" } };
    }
  );
  await hosted.run(ctxReal);
  assert.equal(addonPath, "/api/hosted/v1/subscriptions/5/addons");
  assert.equal(addonBody.sku_id, 5);
  assert.equal(addonBody.quantity, 1);
  assert.equal(addonBody.confirm, true);
  assert.equal(addonBody.idempotency_key, "add-1");
  assert.equal(ctxReal.outputs[0].addon_id, "add-rec-1");
});

test("hosted renew supports dryRun and executes real POST call", async () => {
  // dryRun mode
  let apiCalled = false;
  const ctxDry = mockCtx(
    ["renew", "3"],
    async () => { apiCalled = true; return { success: true }; },
    { dryRun: true }
  );
  await hosted.run(ctxDry);
  assert.equal(apiCalled, false);
  assert.equal(ctxDry.outputs[0].dry_run, true);
  assert.equal(ctxDry.outputs[0].action, "hosted.renew");
  assert.equal(ctxDry.outputs[0].sku_id, 3);

  // real POST call
  let renewPath = "";
  let renewBody = null;
  const ctxReal = mockCtx(
    ["renew", "--sku=3", "--idempotency-key=renew-1"],
    async (method, path, body) => {
      assert.equal(method, "POST");
      renewPath = path;
      renewBody = body;
      return { success: true, data: { subscription_id: "sub-3", status: "renewed" } };
    }
  );
  await hosted.run(ctxReal);
  assert.equal(renewPath, "/api/hosted/v1/subscriptions/3/renew");
  assert.equal(renewBody.sku_id, 3);
  assert.equal(renewBody.idempotency_key, "renew-1");
  assert.equal(ctxReal.outputs[0].subscription_id, "sub-3");
});

test("hosted rejects unknown subcommand and unknown flags", async () => {
  const ctxUnknownSub = mockCtx(["unknown-subcommand"]);
  await assert.rejects(async () => hosted.run(ctxUnknownSub), /unknown hosted subcommand/);

  const ctxUnknownFlag = mockCtx(["status", "--invalid-flag=123"]);
  await assert.rejects(async () => hosted.run(ctxUnknownFlag), /unknown hosted flag: --invalid-flag=123/);
});
