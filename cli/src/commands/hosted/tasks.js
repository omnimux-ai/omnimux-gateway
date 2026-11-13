const path = require("path");

const { UsageError } = require("../../lib/output");
const { guessContentType } = require("../../lib/upload");
const { parseChannelIds, parseScheduledAt } = require("./flags");
const { unwrapApiData, uploadLocalFile } = require("./shared");

async function handleUpload(ctx, positional, flags) {
  const filePath = positional[1] || flags.files[0];
  if (!filePath) {
    throw new UsageError("file path is required (e.g. omnimux hosted upload ./video.mp4)");
  }
  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.upload",
      file: filePath,
      content_type: flags.contentType || guessContentType(filePath) || "application/octet-stream",
    });
    return;
  }
  const result = await uploadLocalFile(ctx, filePath, flags.contentType);
  ctx.out.ok(result);
}

async function handlePost(ctx, positional, flags) {
  const channelId = positional[1] || flags.channel;
  if (!channelId) {
    throw new UsageError("channel id is required (--channel=<id>)");
  }

  let mediaUrl = flags.media;
  if (flags.files && flags.files.length > 0) {
    if (ctx.dryRun) {
      mediaUrl = `https://example.invalid/dry-run/${encodeURIComponent(path.basename(flags.files[0]))}`;
    } else {
      const uploaded = await uploadLocalFile(ctx, flags.files[0], flags.contentType);
      mediaUrl = uploaded.resource_url;
    }
  }

  if (!mediaUrl && !flags.caption) {
    throw new UsageError("either --media/--file or --caption is required for distribution");
  }

  const taskBody = {
    type: "distribute",
    media_url: mediaUrl || "",
    caption: flags.caption || "",
    scheduled_at: parseScheduledAt(flags.scheduledFor),
  };
  if (flags.idempotencyKey) {
    taskBody.idempotency_key = flags.idempotencyKey;
  }

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.post",
      channel_id: Number(channelId),
      task: taskBody,
    });
    return;
  }

  const res = await ctx.api("POST", `/api/hosted/v1/channels/${encodeURIComponent(channelId)}/tasks`, taskBody);
  const data = unwrapApiData(res);
  ctx.out.ok({ channel_id: Number(channelId), ...data });
}

async function handleBatch(ctx, flags) {
  let ids = parseChannelIds(flags.channels);
  if (ids.length === 0 && flags.channel) {
    ids = parseChannelIds(flags.channel);
  }
  if (ids.length === 0) {
    throw new UsageError("at least one channel id is required (--channels=1,2,3)");
  }

  let mediaUrl = flags.media;
  if (flags.files && flags.files.length > 0) {
    if (ctx.dryRun) {
      mediaUrl = `https://example.invalid/dry-run/${encodeURIComponent(path.basename(flags.files[0]))}`;
    } else {
      const uploaded = await uploadLocalFile(ctx, flags.files[0], flags.contentType);
      mediaUrl = uploaded.resource_url;
    }
  }

  if (!mediaUrl && !flags.caption) {
    throw new UsageError("either --media/--file or --caption is required for batch distribution");
  }

  const batchBody = {
    channel_ids: ids,
    media_url: mediaUrl || "",
    caption: flags.caption || "",
    scheduled_at: parseScheduledAt(flags.scheduledFor),
  };
  if (flags.idempotencyKey) {
    batchBody.idempotency_key = flags.idempotencyKey;
  }

  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.batch",
      channels_count: ids.length,
      batch: batchBody,
    });
    return;
  }

  const res = await ctx.api("POST", "/api/hosted/v1/tasks/batch", batchBody);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleTasks(ctx, positional, flags) {
  const channelId = positional[1] || flags.channel;
  if (!channelId) {
    throw new UsageError("channel id is required (--channel=<id> or omnimux hosted tasks <channel_id>)");
  }
  const res = await ctx.api("GET", `/api/hosted/v1/channels/${encodeURIComponent(channelId)}/tasks`);
  const data = unwrapApiData(res);
  ctx.out.ok({ channel_id: Number(channelId), tasks: Array.isArray(data) ? data : [] });
}

async function handleTask(ctx, positional) {
  const taskId = positional[1];
  if (!taskId) {
    throw new UsageError("task id is required (e.g. omnimux hosted task 10)");
  }
  const res = await ctx.api("GET", `/api/hosted/v1/tasks/${encodeURIComponent(taskId)}`);
  const data = unwrapApiData(res);
  ctx.out.ok(data);
}

async function handleCancel(ctx, positional) {
  const taskId = positional[1];
  if (!taskId) {
    throw new UsageError("task id is required (e.g. omnimux hosted cancel 10)");
  }
  if (ctx.dryRun) {
    ctx.out.ok({
      dry_run: true,
      action: "hosted.cancel",
      task_id: Number(taskId),
    });
    return;
  }
  const res = await ctx.api("POST", `/api/hosted/v1/tasks/${encodeURIComponent(taskId)}/cancel`, {});
  const data = unwrapApiData(res);
  ctx.out.ok({ task_id: Number(taskId), ...data });
}

module.exports = {
  handleUpload,
  handlePost,
  handleBatch,
  handleTasks,
  handleTask,
  handleCancel,
};
