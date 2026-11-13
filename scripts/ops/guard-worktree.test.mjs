#!/usr/bin/env node
/**
 * scripts/ops/guard-worktree.test.mjs
 * Behavioural tests for scripts/ops/guard-worktree.mjs (PreToolUse guard).
 *
 * Run: node scripts/ops/guard-worktree.test.mjs
 */

import { spawnSync } from 'node:child_process'
import { existsSync, readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const guard = resolve(here, 'guard-worktree.mjs')
const repo = resolve(here, '..', '..')

/**
 * Fixtures hang off the checkout this test file lives in, so the same suite
 * exercises the guard in the primary checkout and inside a linked worktree.
 * The tracked anchors are files that exist on main, plus one wallet/UI file
 * that only exists on branches that carry the protected frontend paths.
 */
function firstTracked(candidates) {
  for (const candidate of candidates) {
    const absolute = resolve(repo, candidate)
    if (!existsSync(absolute)) continue
    const res = spawnSync('git', ['ls-files', '--error-unmatch', '--', absolute], {
      cwd: repo,
      encoding: 'utf8',
    })
    if (res.status === 0) return candidate
  }
  throw new Error(`no tracked fixture file found under ${repo}`)
}

const TRACKED_GO_FILE = firstTracked(['relay/common/relay_info.go'])
const PROTECTED_UI_FILE = firstTracked([
  'web/src/features/wallet/index.ts',
  'web/src/components/ui/button.tsx',
])

/** A registered sibling worktree, when one exists; from a linked worktree the
 *  primary checkout's .worktrees directory is one level up. */
const LINKED_WORKTREE = (() => {
  const rel = spawnSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], {
    cwd: repo,
    encoding: 'utf8',
  })
  const gitDir = String(rel.stdout || '').trim()
  const dir = gitDir ? resolve(gitDir, '..', '.worktrees') : resolve(repo, '.worktrees')
  try {
    return readdirSync(dir, { withFileTypes: true })
      .filter((entry) => entry.isDirectory() && existsSync(join(dir, entry.name, '.git')))
      .map((entry) => entry.name)[0]
  } catch {
    return undefined
  }
})()

function run(toolName, cwd, toolInput, env) {
  const payload = JSON.stringify({
    hook_event_name: 'PreToolUse',
    tool_name: toolName,
    cwd,
    tool_input: toolInput,
  })
  const res = spawnSync('node', [guard], { input: payload, encoding: 'utf8', env: env || process.env })
  if (res.status !== 0) throw new Error(`guard exited ${res.status}: ${res.stderr}`)
  return JSON.parse(res.stdout).hookSpecificOutput
}

function write(path, cwd = repo) {
  return run('write', cwd, { file_path: path })
}

function edit(path, cwd = repo) {
  return run('edit', cwd, { file_path: path })
}

function bash(command, cwd = repo) {
  return run('bash', cwd, { command })
}

/**
 * Whether this suite runs in the primary checkout. The primary-checkout cases
 * assert refusals; the same refusals are correct only outside a linked worktree,
 * because a worktree is exactly the place the guard lets work happen.
 */
const IS_PRIMARY_CHECKOUT = (() => {
  const common = spawnSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'], {
    cwd: repo,
    encoding: 'utf8',
  })
  const gitDir = spawnSync('git', ['rev-parse', '--absolute-git-dir'], { cwd: repo, encoding: 'utf8' })
  return String(common.stdout || '').trim() === String(gitDir.stdout || '').trim()
})()

const cases = [
  // --- primary checkout only: tracked files must be refused ---
  ...(IS_PRIMARY_CHECKOUT
    ? [
        ['tracked Go source', () => edit(TRACKED_GO_FILE), 'deny'],
        ['tracked file (absolute path)', () => write(resolve(repo, TRACKED_GO_FILE)), 'deny'],
        ['tracked root contract file', () => edit('AGENTS.md'), 'deny'],
        ['tracked git hook', () => edit('.githooks/pre-commit'), 'deny'],
        ['this guard itself (untracked, protected dir)', () => edit('scripts/ops/guard-worktree.mjs'), 'deny'],
        ['new file in docs/', () => write('docs/ops/probe-new-page-zh.md'), 'deny'],
        ['new file in .github/', () => write('.github/workflows/probe.yml'), 'deny'],
        ['new file in deploy/', () => write('deploy/geminix/probe.sh'), 'deny'],
        ['relative cwd still resolves the target', () => run('write', '.', { file_path: 'docs/ops/probe-cwd-relative.md' }), 'deny'],
        ['relative cwd payload still denies a tracked file', () => run('edit', '.', { file_path: TRACKED_GO_FILE }), 'deny'],
        ['foreign cwd payload still denies a tracked absolute path', () => run('edit', '/tmp', { file_path: resolve(repo, 'AGENTS.md') }), 'deny'],
        ['tracked file in protected web/ feature dir is refused', () => edit(PROTECTED_UI_FILE), 'deny'],
      ]
    : [
        ['this suite runs inside a linked worktree, where tracking is irrelevant', () => edit(TRACKED_GO_FILE), 'allow'],
      ]),

  // --- linked worktrees are fully exempt (checked from either location) ---
  ...(LINKED_WORKTREE
    ? [
        ['tracked file inside linked worktree', () => edit(resolve(repo, '.worktrees', LINKED_WORKTREE, TRACKED_GO_FILE)), 'allow'],
        ['new docs file inside linked worktree', () => write(resolve(repo, '.worktrees', LINKED_WORKTREE, 'docs/ops/probe.md')), 'allow'],
      ]
    : []),

  // --- exemptions that hold in both locations ---
  ['untracked draft at repo root', () => write('probe-draft.md'), 'allow'],
  ['ephemeral node_modules path', () => write('web/node_modules/probe/index.js'), 'allow'],
  ['ephemeral tmp path', () => write('.tmp/guard-omnimux/probe.mjs'), 'allow'],
  ['path outside any Git repository', () => write('/tmp/probe-outside.md'), 'allow'],
  ['untracked draft in protected working tree dir is allowed', () => write('web/src/routes/probe-draft.tsx'), 'allow'],

  // --- bash guard: the primary checkout's HEAD may not leave main ---
  ...(IS_PRIMARY_CHECKOUT
    ? [
        ['switch to another branch', () => bash('git switch feat/other'), 'deny'],
        ['checkout spelling of the same switch', () => bash('git checkout feat/other'), 'deny'],
        ['switch -c creates nothing in the primary', () => bash('git switch -c feat/new'), 'deny'],
        ['checkout -b is refused', () => bash('git checkout -b feat/new'), 'deny'],
        ['detaching HEAD is refused', () => bash('git switch --detach'), 'deny'],
        ['checking out a commit is refused', () => bash('git checkout HEAD~2'), 'deny'],
        ['switching to the branch already checked out is still refused', () => bash('git checkout main'), 'deny'],
        ['chained switch after an allowed command is caught', () => bash('git fetch origin && git switch feat/other'), 'deny'],
        ['quoted mention of switch is not a command', () => bash('echo "git switch feat/other"'), 'allow'],
        ['file restore with -- is not a switch', () => bash('git checkout -- relay/common/relay_info.go'), 'allow'],
        ['file restore by path is not a switch', () => bash('git checkout relay/common/relay_utils.go'), 'allow'],
        ['branch listing is not a switch', () => bash('git branch --list "feat/*"'), 'allow'],
        ['creating a branch without switching is not a switch', () => bash('git branch feat/other'), 'allow'],
        ['branch deletion is not a HEAD move', () => bash('git branch -D feat/other'), 'allow'],
        [
          'operator override releases the switch',
          () => run('bash', repo, { command: 'git switch feat/other' }, { ...process.env, OMNIMUX_ALLOW_BRANCH_SWITCH: '1' }),
          'allow',
        ],
      ]
    : [
        ['this suite runs inside a linked worktree, where switching is the workflow', () => bash('git switch feat/other'), 'allow'],
        ['creating a branch inside a linked worktree is allowed', () => bash('git switch -c feat/other'), 'allow'],
      ]),

  // --- bash guard: shell writes get the same verdict as edit/write ---
  ...(IS_PRIMARY_CHECKOUT
    ? [
        ['redirect into a protected dir', () => bash('cat > docs/product/probe.md <<EOF\nhi\nEOF'), 'deny'],
        ['append into a protected dir', () => bash('printf hi >> scripts/ops/probe.mjs'), 'deny'],
        ['cp into a protected dir', () => bash('cp /tmp/a.html docs/product/probe.html'), 'deny'],
        ['mkdir under a protected dir', () => bash('mkdir -p scripts/ops/probe-dir'), 'deny'],
        ['ln into a protected dir', () => bash('ln -s /tmp/x .agents/skills/probe'), 'deny'],
        ['tee into a protected dir', () => bash('echo hi | tee docs/ops/probe.md'), 'deny'],
        ['touch a protected dir file', () => bash('touch deploy/geminix/probe.sh'), 'deny'],
        ['mv a tracked file into a protected dir', () => bash(`mv ${TRACKED_GO_FILE} docs/probe.go`), 'deny'],
        ['sed -i rewrites a tracked file', () => bash(`sed -i 's/a/b/' ${TRACKED_GO_FILE}`), 'deny'],
        ['dd of= a protected dir', () => bash('dd if=/dev/zero of=deploy/geminix/probe.bin count=1'), 'deny'],
        ['chained redirect after an allowed command is caught', () => bash('git status --short && echo x > docs/ops/probe.md'), 'deny'],
      ]
    : [
        ['linked worktree: a protected-dir redirect is the normal workflow', () => bash('cat > docs/product/probe.md <<EOF\nhi\nEOF'), 'allow'],
      ]),

  // --- bash guard: ordinary commands must not be mistaken for writes ---
  ['redirect to /dev/null', () => bash('go build ./... > /dev/null'), 'allow'],
  ['descriptor duplication is not a file', () => bash('make test 2>&1 | tail -5'), 'allow'],
  ['redirect outside the repository', () => bash('npm run build > /tmp/omnimux-build.log'), 'allow'],
  ['redirect into gitignored scratch', () => bash('cat > .scratch/probe.md <<EOF\nhi\nEOF'), 'allow'],
  ['a > inside a commit message is not a redirect', () => bash('git commit -m "fix: a > b"'), 'allow'],
  ['a quoted redirect mention is not a command', () => bash('echo "cat > docs/ops/probe.md"'), 'allow'],
  ['sed without -i only prints', () => bash(`sed 's/a/b/' ${TRACKED_GO_FILE}`), 'allow'],
  ['grep with a pattern containing > stays a read', () => bash('grep -n "=>" relay/common/relay_info.go'), 'allow'],
  ['untracked draft redirect at repo root is still allowed', () => bash('echo hi > probe-draft.md'), 'allow'],
  ['a heredoc body is data, not a command', () => bash('cat > .scratch/probe.sh <<EOF\nmkdir -p docs/probe\necho hi > docs/probe.md\nEOF'), 'allow'],
  ['a quoted heredoc delimiter also ends the body', () => bash("cat > .scratch/probe.md <<'EOF'\nx > docs/probe.md\nEOF"), 'allow'],
  ['a heredoc delimiter with tabs is recognised', () => bash('cat > docs/ops/probe.md <<-EOF\n\thi\n\tEOF'), 'expect-primary-deny'],
  ['nested shell writes are read as commands', () => bash('bash -c "echo hi > docs/ops/probe.md"'), 'expect-nested-shell'],
  ['combined -lc flag is still a nested shell', () => bash('bash -lc "cat > docs/ops/probe.md"'), 'expect-nested-shell'],
  ['nested shell without a write stays allowed', () => bash('bash -c "git status --short"'), 'allow'],

  // --- bash guard: destructive resets against unpushed work ---
  ['git reset --hard', () => bash('git reset --hard HEAD~1'), 'expect-unpushed-or-allow'],
  ['harmless bash command', () => bash('git status --short'), 'allow'],
  ['quoted mention of reset --hard is not a command', () => bash('echo "git reset --hard"'), 'allow'],
  ['empty payload stays allowed', () => run('bash', repo, {}), 'allow'],
  ['empty file_path is ignored', () => run('write', repo, { file_path: '' }), 'allow'],
]

let failed = 0
for (const [name, fn, expected] of cases) {
  const result = fn()
  const decision = result.permissionDecision
  let ok
  if (expected === 'expect-tracked-or-untracked-denial') {
    // The file is tracked in some checkouts and absent in others; both are denials.
    ok = decision === 'deny'
  } else if (expected === 'expect-nested-shell' || expected === 'expect-primary-deny') {
    // A nested shell's write, and a heredoc targeting a protected dir, are only
    // refusals where the write guard applies at all.
    ok = decision === (IS_PRIMARY_CHECKOUT ? 'deny' : 'allow')
  } else if (expected === 'expect-unpushed-or-allow') {
    // Denies only while the branch really carries commits not on origin/main.
    const count = spawnSync('git', ['rev-list', '--count', 'origin/main..HEAD'], { cwd: repo, encoding: 'utf8' })
    const unpushed = Number(String(count.stdout || '0').trim()) || 0
    ok = decision === (unpushed > 0 ? 'deny' : 'allow')
  } else {
    ok = decision === expected
  }
  if (!ok) failed += 1
  const reason = decision === 'deny' ? ` — ${String(result.permissionDecisionReason).split('\n')[0].slice(0, 60)}` : ''
  console.log(`${ok ? 'ok  ' : 'FAIL'}  ${name}: ${decision}${reason}`)
}

console.log(`\n${cases.length - failed}/${cases.length} passed`)
process.exit(failed === 0 ? 0 : 1)
