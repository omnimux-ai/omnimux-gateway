/** Thin command adapters for the owner-scoped creation service. */
const { parseFlags, onlyFlags, requireId, requestId, positiveInt, invalid } = require("../lib/influencer-input");
const client = require("../lib/influencer");
const { download } = require("../lib/influencer-download");
function positionalId(values, label) {
  if (values.length !== 1) throw invalid(`provide one ${label}`);
  return label === "request_id" ? requestId(values[0]) : requireId(values[0], label);
}
/** Artifacts remain downloadable internally; normal output contains safe metadata only. */
function projectOutput(value) {
  if (Array.isArray(value)) return value.map(projectOutput);
  if (!value || typeof value !== "object") return value;
  const projected = {};
  for (const [key, child] of Object.entries(value)) {
    if (key === "legacy_content_url") continue;
    if (key === "artifacts" && Array.isArray(child)) {
      projected.artifacts = child.map((artifact) => Object.fromEntries(
        ["key", "type", "mime_type"].filter((field) => artifact && typeof artifact[field] === "string").map((field) => [field, artifact[field]])
      ));
    } else projected[key] = projectOutput(child);
  }
  return projected;
}
async function run(ctx) {
  const [group, action, ...rest] = ctx.args;
  const simple = group === "options";
  const { flags, positional } = parseFlags(simple ? ctx.args.slice(1) : rest, ctx.flags);
  let data;
  if (simple) {
    onlyFlags(flags, []);
    if (positional.length) throw invalid("options takes no positional arguments");
    data = await client.request(ctx, "GET", "/options");
  } else if (group === "character" && action === "create") {
    if (positional.length) throw invalid("character create takes no positional arguments");
    data = await client.create(ctx, "character", flags);
  } else if (group === "video" && action === "create") {
    if (positional.length) throw invalid("video create takes no positional arguments");
    data = await client.create(ctx, "video", flags);
  } else if (group === "character" && action === "multiview") {
    const id = positionalId(positional, "parent_task_id");
    data = await client.create(ctx, "multiview", flags, id);
  } else if (["task", "character"].includes(group) && action === "list") {
    onlyFlags(flags, group === "task" ? ["page", "page_size", "kind", "status"] : ["page", "page_size"]);
    if (positional.length) throw invalid("list takes no positional arguments");
    const query = new URLSearchParams({ page: positiveInt(flags.page, "--page", 1, 1000000), page_size: positiveInt(flags.page_size, "--page-size", 20, 100) });
    if (flags.kind) {
      if (!["character", "multiview", "video"].includes(flags.kind)) throw invalid("unsupported task kind", "--kind");
      query.set("kind", flags.kind);
    }
    if (flags.status) {
      if (!["waiting", "submitting", "reconciling", "queued", "running", "succeeded", "failed", "unknown"].includes(flags.status)) throw invalid("unsupported task status", "--status");
      query.set("status", flags.status);
    }
    data = await client.request(ctx, "GET", `/${group === "task" ? "tasks" : "characters"}?${query}`);
  } else if (["task", "character", "request"].includes(group) && action === "get") {
    onlyFlags(flags, []);
    const id = positionalId(positional, group === "request" ? "request_id" : "task_id");
    data = group === "request" ? await client.restore(ctx, id) : await client.request(ctx, "GET", `/${group === "task" ? "tasks" : "characters"}/${encodeURIComponent(id)}`);
  } else if (group === "task" && action === "wait") {
    onlyFlags(flags, ["timeout"]);
    const id = positionalId(positional, "task_id");
    data = await client.wait(ctx, id, positiveInt(flags.timeout, "--timeout", 600, 86400));
  } else if (group === "task" && action === "download") {
    onlyFlags(flags, ["output", "overwrite"]);
    const id = positionalId(positional, "task_id");
    if (!flags.output) throw invalid("--output is required", "--output");
    data = await download(ctx, id, flags.output, Boolean(flags.overwrite));
  } else throw invalid("unknown influencer action; use omnimux schema");
  ctx.out.ok(projectOutput(data));
}
module.exports = { run };
