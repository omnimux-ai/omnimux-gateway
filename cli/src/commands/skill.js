/**
 * omnimux skill — install / update / status for AI editor skills (flat surface).
 *
 * Scope:
 *   --user     ~/.agents/skills/<skillId>
 *   --project  <cwd>/.agents/skills/<skillId>
 *
 * Mode:
 *   --mode=link  (default) symlink each skill → package skill/<id>
 *   --mode=copy  full copy + .omnimux-skill-meta.json
 *
 * Optional:
 *   --only=id1,id2   install subset (default: all skills in package manifest)
 */

const { printJson, UsageError } = require("../lib/output");
const { ApiError, ConfigError } = require("../lib/errors");
const skill = require("../lib/skill");

function emitOk(ctx, data) {
  if (ctx.out) ctx.out.ok(data);
  else printJson(data, ctx.flags);
}

function failIfUnsuccessful(result, fallbackMessage) {
  if (result && result.success === false) {
    throw new ApiError(
      String((result && result.error) || fallbackMessage || "skill operation failed"),
      { subtype: "upstream" }
    );
  }
}

/**
 * @param {string[]} args
 * @param {Record<string, unknown>} flags
 */
function parseSkillArgs(args, flags) {
  const out = {
    sub: args[0] || "status",
    scope: null,
    mode: flags.mode || "auto",
    force: !!flags.force,
    only: null,
    rest: [],
  };
  for (let i = 1; i < args.length; i++) {
    const a = args[i];
    if (a === "--user") out.scope = "user";
    else if (a === "--project") out.scope = "project";
    else if (a === "--force") out.force = true;
    else if (a.startsWith("--mode=")) out.mode = a.slice("--mode=".length);
    else if (a === "--mode") out.mode = args[++i];
    else if (a.startsWith("--only=")) {
      out.only = a
        .slice("--only=".length)
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
    } else if (a === "--only") {
      out.only = String(args[++i] || "")
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
    } else out.rest.push(a);
  }
  if (flags.user) out.scope = "user";
  if (flags.project) out.scope = "project";
  if (flags.mode) out.mode = flags.mode;
  if (flags.only) {
    out.only = String(flags.only)
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  }
  return out;
}

/**
 * @param {{ flags: Record<string, unknown>, args: string[] }} ctx
 */
async function run(ctx) {
  const parsed = parseSkillArgs(ctx.args || [], ctx.flags || {});
  const sub = String(parsed.sub || "status").toLowerCase();

  if (sub === "status" || sub === "list") {
    emitOk(ctx, skill.skillStatus({ cwd: process.cwd() }));
    return;
  }

  if (sub === "read") {
    const skillId = (parsed.rest && parsed.rest[0]) || "";
    if (!skillId) {
      throw new UsageError("usage: omnimux skill read <skillId>", {
        recovery: { command: "omnimux", args: ["skill", "list"] },
      });
    }
    const result = skill.readSkill(skillId, { cwd: process.cwd() });
    if (!result.success) {
      if (result.code === "skill_tree_missing") {
        throw new ConfigError(result.error, {
          subtype: "not_configured",
          hint: "install the npm package so skill/ ships next to the CLI, or set OMNIMUX_SKILL_SOURCE",
          recovery: {
            command: "omnimux",
            args: ["update", "--check"],
          },
        });
      }
      if (result.code === "unknown_skill" || result.code === "missing_id") {
        throw new UsageError(result.error, {
          subtype: "invalid_argument",
          recovery: { command: "omnimux", args: ["skill", "list"] },
        });
      }
      throw new ApiError(result.error || "skill read failed", { subtype: "upstream" });
    }
    emitOk(ctx, result);
    return;
  }

  if (sub === "install") {
    if (!parsed.scope) {
      throw new UsageError(
        "skill install requires --user or --project",
        {
          hint: "pass --user or --project",
          recovery: { command: "omnimux", args: ["skill", "install", "--user"] },
        }
      );
    }
    const result = skill.installSkill({
      scope: parsed.scope,
      mode: parsed.mode === "copy" || parsed.mode === "link" ? parsed.mode : "auto",
      cwd: process.cwd(),
      skillIds: parsed.only || undefined,
    });
    failIfUnsuccessful(result, "skill install failed");
    emitOk(ctx, result);
    return;
  }

  if (sub === "update" || sub === "sync") {
    const scopes = parsed.scope ? [parsed.scope] : undefined;
    // If scope given, force reinstall even when empty discovery
    if (parsed.scope) {
      const result = skill.installSkill({
        scope: parsed.scope,
        mode: parsed.mode === "copy" || parsed.mode === "link" ? parsed.mode : "auto",
        cwd: process.cwd(),
        skillIds: parsed.only || undefined,
      });
      failIfUnsuccessful(result, "skill update failed");
      emitOk(ctx, {
        success: true,
        action: "synced",
        results: [result],
      });
      return;
    }
    const result = skill.syncAllSkills({
      scopes,
      cwd: process.cwd(),
      includeDiscovered: true,
      skillIds: parsed.only || undefined,
    });
    failIfUnsuccessful(result, "skill sync failed");
    emitOk(ctx, result);
    return;
  }

  if (sub === "uninstall" || sub === "remove") {
    if (!parsed.scope) {
      throw new UsageError("skill uninstall requires --user or --project");
    }
    const onlyId =
      parsed.only && parsed.only.length === 1 ? parsed.only[0] : undefined;
    const result = skill.uninstallSkill(parsed.scope, {
      cwd: process.cwd(),
      skillId: onlyId,
    });
    failIfUnsuccessful(result, "skill uninstall failed");
    emitOk(ctx, result);
    return;
  }

  throw new UsageError(
    `unknown skill subcommand: ${sub}`,
    {
      hint: "status|list|read|install|update|uninstall",
      recovery: { command: "omnimux", args: ["skill", "status"] },
    }
  );
}

module.exports = { run, parseSkillArgs };
