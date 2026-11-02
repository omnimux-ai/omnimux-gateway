/**
 * OmniMux CLI skill install / sync (flat Agent discovery surface).
 *
 * Package truth: <package>/skill/<skillId>/SKILL.md (+ manifest.json)
 * Agent surface: ~/.agents/skills/<skillId> or <repo>/.agents/skills/<skillId>
 *
 * Registry: ~/.config/omnimux/skill-targets.json (schema 2: one row per skill_id)
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const { configDir, isBlank } = require("./config");

const META_FILENAME = ".omnimux-skill-meta.json";
const SHARED_SKILL_ID = "omnimux";
const REGISTRY_SCHEMA = 2;
const PKG = require("../../package.json");

function exists(p) {
  try {
    fs.accessSync(p);
    return true;
  } catch {
    return false;
  }
}

function realpathOrSelf(p) {
  try {
    return fs.realpathSync(p);
  } catch {
    return p;
  }
}

function resolveHomeDir() {
  if (!isBlank(process.env.OMNIMUX_HOME)) {
    return path.resolve(String(process.env.OMNIMUX_HOME));
  }
  if (!isBlank(process.env.HOME)) {
    return path.resolve(String(process.env.HOME));
  }
  if (!isBlank(process.env.USERPROFILE)) {
    return path.resolve(String(process.env.USERPROFILE));
  }
  return os.homedir();
}

/**
 * Package skill root: directory containing manifest.json and skill subdirs.
 * @param {{ argv?: string[] }} [proc]
 */
function resolvePackageSkillRoot(proc = process) {
  if (!isBlank(process.env.OMNIMUX_SKILL_SOURCE)) {
    const skillRoot = path.resolve(String(process.env.OMNIMUX_SKILL_SOURCE));
    if (isSkillRoot(skillRoot)) {
      return { skillRoot, packageRoot: path.dirname(skillRoot), source: "env" };
    }
    return {
      skillRoot: null,
      packageRoot: null,
      source: "env",
      error: `OMNIMUX_SKILL_SOURCE is not a skill root (need manifest.json or */SKILL.md): ${skillRoot}`,
    };
  }

  const fromModule = path.resolve(__dirname, "..", "..", "skill");
  if (isSkillRoot(fromModule)) {
    return {
      skillRoot: fromModule,
      packageRoot: path.resolve(__dirname, "..", ".."),
      source: "package",
    };
  }

  const argv = Array.isArray(proc.argv) ? proc.argv : [];
  const argv1 = argv[1] ? realpathOrSelf(String(argv[1])) : "";
  const argv1Norm = argv1.replace(/\\/g, "/");
  const m = argv1Norm.match(/^(.*\/(?:node_modules\/)?@omnimux\/cli)(?:\/|$)/);
  if (m) {
    const packageRoot = m[1];
    const skillRoot = path.join(packageRoot, "skill");
    if (isSkillRoot(skillRoot)) {
      return { skillRoot, packageRoot, source: "argv" };
    }
  }

  if (argv1Norm.endsWith("/src/bin.js") || argv1Norm.endsWith("/cli/src/bin.js")) {
    const packageRoot = path.resolve(path.dirname(argv1), "..");
    const skillRoot = path.join(packageRoot, "skill");
    if (isSkillRoot(skillRoot)) {
      return { skillRoot, packageRoot, source: "argv" };
    }
  }

  return {
    skillRoot: null,
    packageRoot: null,
    source: "missing",
    error:
      "skill tree not found next to this CLI install (use npm install -g @omnimux/cli; binary builds do not embed skill/)",
  };
}

/** @deprecated use resolvePackageSkillRoot */
function resolvePackageSkillSource(proc = process) {
  const r = resolvePackageSkillRoot(proc);
  return {
    root: r.packageRoot,
    skillRoot: r.skillRoot,
    source: r.source,
    error: r.error,
  };
}

function isSkillRoot(dir) {
  if (!exists(dir)) return false;
  if (exists(path.join(dir, "manifest.json"))) return true;
  try {
    for (const name of fs.readdirSync(dir)) {
      if (name.startsWith(".")) continue;
      if (exists(path.join(dir, name, "SKILL.md"))) return true;
    }
  } catch {
    return false;
  }
  return false;
}

/**
 * @param {string} skillRoot
 * @returns {Array<{ id: string, path: string, role: string, required?: boolean, requires?: string[] }>}
 */
function listPackageSkills(skillRoot) {
  const manifestPath = path.join(skillRoot, "manifest.json");
  if (exists(manifestPath)) {
    try {
      const raw = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
      if (raw && Array.isArray(raw.skills)) {
        return raw.skills
          .filter((s) => s && !isBlank(s.id) && !isBlank(s.path))
          .map((s) => ({
            id: String(s.id).trim(),
            path: String(s.path).trim(),
            role: s.role === "shared" ? "shared" : "business",
            required: !!s.required,
            requires: Array.isArray(s.requires) ? s.requires.map(String) : [],
          }));
      }
    } catch {
      // fall through to scan
    }
  }

  const out = [];
  for (const name of fs.readdirSync(skillRoot)) {
    if (name.startsWith(".") || name === "README.md" || name === "manifest.json") continue;
    const abs = path.join(skillRoot, name);
    try {
      if (!fs.statSync(abs).isDirectory()) continue;
    } catch {
      continue;
    }
    if (!exists(path.join(abs, "SKILL.md"))) continue;
    out.push({
      id: name,
      path: name,
      role: name === SHARED_SKILL_ID ? "shared" : "business",
      required: name === SHARED_SKILL_ID,
      requires: name === SHARED_SKILL_ID ? [] : [SHARED_SKILL_ID],
    });
  }
  return out.sort((a, b) => a.id.localeCompare(b.id));
}

function skillDirInPackage(skillRoot, skillPathRel) {
  return path.join(skillRoot, skillPathRel);
}

function skillTargetsPath() {
  return path.join(configDir(), "skill-targets.json");
}

/**
 * @returns {{ schema: number, targets: Array<Record<string, unknown>> }}
 */
function loadSkillTargets() {
  const p = skillTargetsPath();
  if (!exists(p)) return { schema: REGISTRY_SCHEMA, targets: [] };
  try {
    const raw = JSON.parse(fs.readFileSync(p, "utf8"));
    if (!raw || typeof raw !== "object") return { schema: REGISTRY_SCHEMA, targets: [] };
    const targets = Array.isArray(raw.targets) ? raw.targets : [];
    const schema = typeof raw.schema === "number" ? raw.schema : 1;
    return {
      schema,
      targets: targets.filter((t) => t && typeof t === "object" && !isBlank(t.path)),
    };
  } catch {
    return { schema: REGISTRY_SCHEMA, targets: [] };
  }
}

/**
 * @param {{ schema?: number, targets: Array<Record<string, unknown>> }} data
 */
function saveSkillTargets(data) {
  const dir = configDir();
  if (!exists(dir)) {
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
  }
  const clean = {
    schema: REGISTRY_SCHEMA,
    targets: (data.targets || []).map((t) => ({
      scope: String(t.scope || "user"),
      skill_id: String(t.skill_id || SHARED_SKILL_ID),
      path: String(t.path),
      mode: t.mode === "link" ? "link" : "copy",
      project_root: t.project_root ? String(t.project_root) : undefined,
      cli_version: t.cli_version ? String(t.cli_version) : undefined,
      updated_at: t.updated_at ? String(t.updated_at) : undefined,
    })),
  };
  fs.writeFileSync(skillTargetsPath(), JSON.stringify(clean, null, 2) + "\n", {
    mode: 0o600,
  });
  return skillTargetsPath();
}

/**
 * @param {"user"|"project"} scope
 * @param {string} skillId
 * @param {string} [cwd]
 */
function defaultTargetPath(scope, skillId, cwd = process.cwd()) {
  const id = skillId || SHARED_SKILL_ID;
  if (scope === "user") {
    return path.join(resolveHomeDir(), ".agents", "skills", id);
  }
  return path.join(path.resolve(cwd), ".agents", "skills", id);
}

/**
 * True if path looks like 0.2.6 whole-tree install (hub + nested packs).
 * @param {string} targetPath
 */
function isLegacyBundleInstall(targetPath) {
  if (!exists(targetPath)) return false;
  if (exists(path.join(targetPath, "packs"))) return true;
  // Root SKILL.md that is the old hub sitting at skills/omnimux with sibling only packs
  const resolved = realpathOrSelf(targetPath);
  if (exists(path.join(resolved, "packs"))) return true;
  // Link pointing at package skill/ root (contains multiple skill dirs + no single-skill identity)
  try {
    const st = fs.lstatSync(targetPath);
    if (st.isSymbolicLink()) {
      const dest = realpathOrSelf(targetPath);
      if (exists(path.join(dest, "manifest.json"))) return true;
      if (exists(path.join(dest, "omnimux", "SKILL.md")) && exists(path.join(dest, "recreate-viral-video", "SKILL.md"))) {
        return true;
      }
      // Old package skill root: SKILL.md at root + packs/
      if (exists(path.join(dest, "SKILL.md")) && exists(path.join(dest, "packs"))) return true;
      if (exists(path.join(dest, "SKILL.md")) && exists(path.join(dest, "packs", "recreate-viral-video"))) return true;
    }
  } catch {
    /* ignore */
  }
  return false;
}

function readTargetMeta(targetPath) {
  const metaPath = path.join(targetPath, META_FILENAME);
  if (!exists(metaPath)) return null;
  try {
    return JSON.parse(fs.readFileSync(metaPath, "utf8"));
  } catch {
    return null;
  }
}

function inspectTarget(targetPath) {
  const p = path.resolve(targetPath);
  if (!exists(p)) {
    return { path: p, exists: false, kind: "missing", meta: null, link_target: null };
  }
  let kind = "dir";
  let linkTarget = null;
  try {
    const st = fs.lstatSync(p);
    if (st.isSymbolicLink()) {
      kind = "link";
      try {
        linkTarget = fs.readlinkSync(p);
      } catch {
        linkTarget = null;
      }
    } else if (!st.isDirectory()) {
      kind = "other";
    }
  } catch {
    return { path: p, exists: false, kind: "missing", meta: null, link_target: null };
  }
  return {
    path: p,
    exists: true,
    kind,
    has_skill_md: exists(path.join(p, "SKILL.md")),
    legacy_bundle: isLegacyBundleInstall(p),
    meta: readTargetMeta(p),
    link_target: linkTarget,
    resolved: kind === "link" ? realpathOrSelf(p) : p,
  };
}

function removePathSync(targetPath) {
  if (!exists(targetPath)) return;
  const st = fs.lstatSync(targetPath);
  if (st.isSymbolicLink() || st.isFile()) {
    fs.unlinkSync(targetPath);
    return;
  }
  fs.rmSync(targetPath, { recursive: true, force: true });
}

function copyDirSync(src, dest) {
  if (!exists(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  } else {
    const st = fs.lstatSync(dest);
    if (!st.isDirectory() || st.isSymbolicLink()) {
      removePathSync(dest);
      fs.mkdirSync(dest, { recursive: true });
    }
  }
  for (const ent of fs.readdirSync(src, { withFileTypes: true })) {
    const from = path.join(src, ent.name);
    const to = path.join(dest, ent.name);
    if (ent.isDirectory()) {
      copyDirSync(from, to);
    } else if (ent.isSymbolicLink()) {
      const link = fs.readlinkSync(from);
      try {
        fs.lstatSync(to);
        removePathSync(to);
      } catch {
        /* missing */
      }
      try {
        fs.symlinkSync(link, to);
      } catch {
        if (exists(from)) fs.copyFileSync(from, to);
      }
    } else if (ent.isFile()) {
      fs.copyFileSync(from, to);
    }
  }
}

/**
 * Install one skill id into the Agent discovery surface.
 */
function installOneSkill(options) {
  const scope = options.scope;
  const skillId = String(options.skillId);
  const skillPathRel = String(options.skillPathRel || skillId);
  const sourceRoot = options.skillRoot;
  const srcDir = skillDirInPackage(sourceRoot, skillPathRel);
  if (!exists(path.join(srcDir, "SKILL.md"))) {
    return {
      success: false,
      skill_id: skillId,
      error: `package skill missing SKILL.md: ${srcDir}`,
    };
  }

  const targetPath = path.resolve(
    options.targetPath || defaultTargetPath(scope, skillId, options.cwd || process.cwd())
  );
  let mode = options.mode || "auto";
  if (mode === "auto") mode = "link";

  fs.mkdirSync(path.dirname(targetPath), { recursive: true });
  removePathSync(targetPath);

  let usedMode = mode;
  let warning = null;
  if (mode === "link") {
    try {
      fs.symlinkSync(srcDir, targetPath, "dir");
    } catch (err) {
      usedMode = "copy";
      warning =
        err && err.message
          ? `symlink failed (${err.message}); fell back to copy`
          : "symlink failed; fell back to copy";
      copyDirSync(srcDir, targetPath);
    }
  } else {
    copyDirSync(srcDir, targetPath);
  }

  const cliVersion = options.cliVersion || PKG.version;
  const updatedAt = new Date().toISOString();
  if (usedMode === "copy") {
    fs.writeFileSync(
      path.join(targetPath, META_FILENAME),
      JSON.stringify(
        {
          skill_id: skillId,
          cli_version: cliVersion,
          mode: usedMode,
          scope,
          source_skill_dir: srcDir,
          updated_at: updatedAt,
        },
        null,
        2
      ) + "\n",
      { mode: 0o644 }
    );
  }

  return {
    success: true,
    skill_id: skillId,
    scope,
    mode: usedMode,
    path: targetPath,
    source_skill_dir: srcDir,
    cli_version: cliVersion,
    warning,
    inspect: inspectTarget(targetPath),
    _registry: {
      scope,
      skill_id: skillId,
      path: targetPath,
      mode: usedMode,
      project_root: scope === "project" ? path.resolve(options.cwd || process.cwd()) : undefined,
      cli_version: cliVersion,
      updated_at: updatedAt,
    },
  };
}

/**
 * Install all (or selected) package skills for a scope — flat Agent surface.
 * @param {{
 *   scope: "user"|"project",
 *   mode?: "link"|"copy"|"auto",
 *   cwd?: string,
 *   skillIds?: string[],
 *   cliVersion?: string,
 *   proc?: NodeJS.Process,
 * }} options
 */
function installSkill(options) {
  const scope = options.scope;
  if (scope !== "user" && scope !== "project") {
    return { success: false, error: "scope must be user or project" };
  }

  const rootInfo = resolvePackageSkillRoot(options.proc || process);
  if (!rootInfo.skillRoot) {
    return { success: false, error: rootInfo.error || "skill source missing", source: rootInfo };
  }

  // Drop legacy whole-tree install at skills/omnimux if present
  const legacyPath = defaultTargetPath(scope, SHARED_SKILL_ID, options.cwd || process.cwd());
  let legacyMigrated = false;
  if (isLegacyBundleInstall(legacyPath)) {
    removePathSync(legacyPath);
    legacyMigrated = true;
  }

  const catalog = listPackageSkills(rootInfo.skillRoot);
  let selected = catalog;
  if (options.skillIds && options.skillIds.length) {
    const want = new Set(options.skillIds.map(String));
    selected = catalog.filter((s) => want.has(s.id));
    if (selected.length === 0) {
      return {
        success: false,
        error: `no matching skills for --only; available: ${catalog.map((s) => s.id).join(", ")}`,
      };
    }
  }

  const mode = options.mode === "copy" || options.mode === "link" ? options.mode : "auto";
  const installed = [];
  for (const s of selected) {
    const one = installOneSkill({
      scope,
      skillId: s.id,
      skillPathRel: s.path,
      skillRoot: rootInfo.skillRoot,
      mode,
      cwd: options.cwd || process.cwd(),
      cliVersion: options.cliVersion || PKG.version,
    });
    installed.push(one);
  }

  const failed = installed.filter((r) => !r.success);
  const registry = loadSkillTargets();
  const keepOthers = registry.targets.filter((t) => {
    if (String(t.scope) !== scope) return true;
    if (scope === "project") {
      const pr = path.resolve(options.cwd || process.cwd());
      if (t.project_root && path.resolve(String(t.project_root)) !== pr) return true;
      // same project: drop ids we reinstalled or legacy omnimux without skill_id
      const id = t.skill_id ? String(t.skill_id) : SHARED_SKILL_ID;
      if (selected.some((s) => s.id === id)) return false;
      if (!t.skill_id && String(t.path) === legacyPath) return false;
      return true;
    }
    // user scope: drop reinstalled ids
    const id = t.skill_id ? String(t.skill_id) : SHARED_SKILL_ID;
    if (selected.some((s) => s.id === id)) return false;
    if (!t.skill_id) return false; // drop schema1 user rows
    return true;
  });
  for (const r of installed) {
    if (r.success && r._registry) keepOthers.push(r._registry);
  }
  saveSkillTargets({ targets: keepOthers });

  return {
    success: failed.length === 0,
    action: legacyMigrated ? "migrated_and_installed" : "installed",
    scope,
    mode: mode === "auto" ? "link" : mode,
    legacy_bundle_migrated: legacyMigrated,
    source_skill_root: rootInfo.skillRoot,
    cli_version: options.cliVersion || PKG.version,
    installed: installed.map((r) => {
      const { _registry, ...rest } = r;
      return rest;
    }),
    error: failed.length ? `${failed.length} skill(s) failed` : undefined,
  };
}

function syncAllSkills(options = {}) {
  const rootInfo = resolvePackageSkillRoot(options.proc || process);
  if (!rootInfo.skillRoot) {
    return {
      success: false,
      error: rootInfo.error || "skill source missing",
      source: rootInfo,
      results: [],
    };
  }

  const cwd = options.cwd || process.cwd();
  const scopes = options.scopes && options.scopes.length ? options.scopes : ["user", "project"];
  const results = [];

  for (const scope of scopes) {
    if (scope !== "user" && scope !== "project") continue;
    // Prefer reinstall-all from package catalog for that scope when any install exists or includeDiscovered
    const legacy = defaultTargetPath(scope, SHARED_SKILL_ID, cwd);
    const hasAny =
      isLegacyBundleInstall(legacy) ||
      listPackageSkills(rootInfo.skillRoot).some((s) =>
        exists(defaultTargetPath(scope, s.id, cwd))
      ) ||
      loadSkillTargets().targets.some((t) => String(t.scope) === scope);

    if (!hasAny && options.includeDiscovered === false) continue;
    if (!hasAny && options.includeDiscovered !== false) {
      // only install if user already has agents skills dir partial? skip empty scopes
      continue;
    }

    // If scope has registry entries or discovered dirs, reinstall full catalog for that scope
    const modeHint = loadSkillTargets().targets.find(
      (t) => String(t.scope) === scope && t.mode === "copy"
    )
      ? "copy"
      : "link";

    const r = installSkill({
      scope,
      mode: modeHint,
      cwd: scope === "project" ? cwd : cwd,
      cliVersion: options.cliVersion || PKG.version,
      proc: options.proc,
      skillIds: options.skillIds,
    });
    results.push(r);
  }

  // If nothing was registered but includeDiscovered and legacy/project path exists
  if (results.length === 0 && options.includeDiscovered !== false) {
    for (const scope of scopes) {
      const p = defaultTargetPath(scope, SHARED_SKILL_ID, cwd);
      if (exists(p) || isLegacyBundleInstall(p)) {
        results.push(
          installSkill({
            scope,
            mode: "auto",
            cwd,
            cliVersion: options.cliVersion || PKG.version,
            proc: options.proc,
          })
        );
      }
    }
  }

  const failed = results.filter((r) => !r.success);
  return {
    success: failed.length === 0,
    action: "synced",
    source_skill_root: rootInfo.skillRoot,
    cli_version: options.cliVersion || PKG.version,
    results,
    error: failed.length ? `${failed.length} scope(s) failed` : undefined,
  };
}

function skillStatus(options = {}) {
  const rootInfo = resolvePackageSkillRoot(options.proc || process);
  const cwd = options.cwd || process.cwd();
  const catalog = rootInfo.skillRoot ? listPackageSkills(rootInfo.skillRoot) : [];
  const registry = loadSkillTargets();

  const agentSkills = [];
  for (const scope of ["user", "project"]) {
    for (const s of catalog.length ? catalog : [{ id: SHARED_SKILL_ID }]) {
      const p = defaultTargetPath(scope, s.id, cwd);
      const insp = inspectTarget(p);
      if (insp.exists || insp.legacy_bundle) {
        agentSkills.push({
          skill_id: s.id,
          scope,
          path: p,
          inspect: insp,
        });
      }
    }
  }

  // Also surface registry rows whose path differs
  for (const t of registry.targets) {
    const p = String(t.path);
    if (agentSkills.some((a) => a.path === p)) continue;
    agentSkills.push({
      skill_id: t.skill_id || "(unknown)",
      scope: t.scope,
      path: p,
      inspect: inspectTarget(p),
      from_registry: true,
    });
  }

  const drifts = [];
  for (const a of agentSkills) {
    const insp = a.inspect;
    if (!insp || !insp.exists) continue;
    if (insp.legacy_bundle) {
      drifts.push({
        path: a.path,
        skill_id: a.skill_id,
        reason: "legacy_bundle_install",
        hint: "run: omnimux skill install --" + a.scope,
      });
      continue;
    }
    if (insp.kind === "link" && rootInfo.skillRoot) {
      const expected = path.join(rootInfo.skillRoot, String(a.skill_id));
      if (insp.resolved && realpathOrSelf(insp.resolved) !== realpathOrSelf(expected)) {
        // skill_id may not match catalog if old bundle
        const ok = catalog.some(
          (s) => realpathOrSelf(path.join(rootInfo.skillRoot, s.path)) === realpathOrSelf(insp.resolved)
        );
        if (!ok) {
          drifts.push({
            path: a.path,
            skill_id: a.skill_id,
            reason: "link_target_not_in_package",
            resolved: insp.resolved,
          });
        }
      }
    }
    const metaVer = insp.meta && insp.meta.cli_version;
    if (metaVer && metaVer !== PKG.version) {
      drifts.push({
        path: a.path,
        skill_id: a.skill_id,
        reason: "copy_version_behind",
        installed_cli_version: metaVer,
        current_cli_version: PKG.version,
      });
    }
  }

  return {
    success: true,
    cli_version: PKG.version,
    registry_schema: registry.schema,
    package_skill: rootInfo,
    catalog: catalog.map((s) => ({ id: s.id, role: s.role, requires: s.requires })),
    agent_skills: agentSkills,
    drifts,
    recommendation:
      drifts.some((d) => d.reason === "legacy_bundle_install")
        ? "Legacy whole-tree install detected. Run: omnimux skill install --user and/or --project"
        : drifts.length
          ? "Run: omnimux skill update"
          : agentSkills.length === 0
            ? "Run: omnimux skill install --user   (or --project)"
            : undefined,
  };
}

/**
 * Uninstall all skills for a scope (or one skill_id).
 * @param {"user"|"project"} scope
 * @param {{ cwd?: string, skillId?: string }} [options]
 */
/**
 * Read a package skill's SKILL.md (version-locked to this CLI install).
 * Binary-only installs without an adjacent skill/ tree fail with a typed error
 * payload (caller should throw ConfigError/ApiError from the command layer).
 *
 * @param {string} skillId
 * @param {{ proc?: NodeJS.Process, cwd?: string }} [options]
 * @returns {{ success: true, skill_id: string, path: string, content: string, cli_version: string } | { success: false, error: string, code: string }}
 */
function readSkill(skillId, options = {}) {
  const id = String(skillId || "").trim();
  if (!id) {
    return { success: false, error: "skill id is required", code: "missing_id" };
  }
  const rootInfo = resolvePackageSkillRoot(options.proc || process);
  if (!rootInfo.skillRoot) {
    return {
      success: false,
      error:
        rootInfo.error ||
        "skill tree not found next to this CLI install (binary builds do not embed skill/)",
      code: "skill_tree_missing",
      package_skill: rootInfo,
    };
  }
  const catalog = listPackageSkills(rootInfo.skillRoot);
  const entry = catalog.find((s) => s.id === id);
  if (!entry) {
    return {
      success: false,
      error: `unknown skill id: ${id}`,
      code: "unknown_skill",
      available: catalog.map((s) => s.id),
    };
  }
  const skillMd = path.join(rootInfo.skillRoot, entry.path, "SKILL.md");
  if (!exists(skillMd)) {
    return {
      success: false,
      error: `SKILL.md missing for ${id}`,
      code: "missing_skill_md",
      path: skillMd,
    };
  }
  let content;
  try {
    content = fs.readFileSync(skillMd, "utf8");
  } catch (err) {
    return {
      success: false,
      error: err && err.message ? err.message : "failed to read SKILL.md",
      code: "read_failed",
      path: skillMd,
    };
  }
  return {
    success: true,
    skill_id: id,
    path: skillMd,
    content,
    cli_version: PKG.version,
    package_skill_source: rootInfo.source,
  };
}

function uninstallSkill(scope, options = {}) {
  const cwd = options.cwd || process.cwd();
  const rootInfo = resolvePackageSkillRoot();
  const catalog = rootInfo.skillRoot ? listPackageSkills(rootInfo.skillRoot) : [];
  const ids = options.skillId
    ? [options.skillId]
    : catalog.length
      ? catalog.map((s) => s.id)
      : [SHARED_SKILL_ID];

  const removed = [];
  for (const id of ids) {
    const p = defaultTargetPath(scope, id, cwd);
    if (exists(p) || isLegacyBundleInstall(p)) {
      removePathSync(p);
      removed.push(p);
    }
  }
  // legacy bundle path
  const legacy = defaultTargetPath(scope, SHARED_SKILL_ID, cwd);
  if (exists(legacy)) {
    removePathSync(legacy);
    if (!removed.includes(legacy)) removed.push(legacy);
  }

  const registry = loadSkillTargets();
  registry.targets = registry.targets.filter((t) => {
    if (String(t.scope) !== scope) return true;
    if (options.skillId) return String(t.skill_id) !== options.skillId;
    if (scope === "project" && t.project_root) {
      return path.resolve(String(t.project_root)) !== path.resolve(cwd);
    }
    return false;
  });
  saveSkillTargets(registry);

  return {
    success: true,
    action: "uninstalled",
    scope,
    removed,
  };
}

module.exports = {
  META_FILENAME,
  SHARED_SKILL_ID,
  REGISTRY_SCHEMA,
  DEFAULT_SKILL_DIR_NAME: SHARED_SKILL_ID,
  resolveHomeDir,
  resolvePackageSkillRoot,
  resolvePackageSkillSource,
  listPackageSkills,
  loadSkillTargets,
  saveSkillTargets,
  defaultTargetPath,
  isLegacyBundleInstall,
  inspectTarget,
  installSkill,
  installOneSkill,
  syncAllSkills,
  skillStatus,
  readSkill,
  uninstallSkill,
  copyDirSync,
};
