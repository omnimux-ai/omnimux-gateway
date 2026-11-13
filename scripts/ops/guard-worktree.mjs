#!/usr/bin/env node
/**
 * scripts/ops/guard-worktree.mjs
 * dsh-hooks-plugin PreToolUse Guard for the OmniMux fork.
 *
 * Implements AGENTS.md "Hard bounds (fork & git)": the primary checkout on main
 * stays clean. Every tracked-file edit must happen inside a linked worktree,
 * which is created and entered with ./scripts/ops/worktree.sh new <task>.
 *
 * The guard also intercepts destructive resets that would overwrite commits
 * which are not yet pushed to origin/main.
 *
 * Exemptions (writes allowed in the primary checkout):
 *   - linked worktree directories (verified against Git metadata + worktree registry)
 *   - ephemeral or derived paths: node_modules, dist, tmp, temp, coverage, *.log, *.tsbuildinfo
 *   - .gitignore'd files, including untracked local drafts (git check-ignore)
 *   - paths outside any Git repository
 */

import { spawnSync } from 'node:child_process'
import { lstatSync, realpathSync, readFileSync } from 'node:fs'
import { dirname, isAbsolute, relative, resolve, sep } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const WORKTREE_COMMAND = './scripts/ops/worktree.sh new <task>'

/** The checkout this guard ships with; payload cwd resolution falls back to it. */
const scriptRepo = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')

const EPHEMERAL_DIR_NAMES = new Set([
  'node_modules',
  'dist',
  'dist-harness',
  'tmp',
  'temp',
  '.tmp',
  '.cache',
  'coverage',
  '.pnpm-store',
])

const EPHEMERAL_BASENAMES = new Set(['.DS_Store'])
const EPHEMERAL_SUFFIXES = ['.log', '.tsbuildinfo']

/** Directories whose untracked additions would pollute the primary checkout. */
const PROTECTED_DIR_PREFIXES = [
  '.githooks/',
  '.github/',
  '.agents/',
  'web/src/features/wallet/',
  'web/src/features/auth/',
  'deploy/',
  'scripts/',
  'docs/',
]

/** Always-on contract files at the repository root. */
const PROTECTED_ROOT_FILES = new Set([
  'AGENTS.md',
  'CLAUDE.md',
  'DESIGN.md',
  'FORK.md',
  'FORK_CUSTOMIZATIONS.md',
  '.githooks/pre-commit',
  '.githooks/commit-msg',
  '.dsh/hooks.json',
  '.dsh/hooks.local.json',
  'go.mod',
  'Makefile',
])

/** Resolve symlinks before classifying existing files or a new file's parent. */
function canonicalTarget(fullPath) {
  let ancestor = resolve(fullPath)
  const missing = []
  for (;;) {
    try {
      lstatSync(ancestor)
      return resolve(realpathSync(ancestor), ...missing)
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
      // A dangling symlink must not be mistaken for a missing directory.
      try {
        if (lstatSync(ancestor).isSymbolicLink()) throw new Error('Dangling symlink')
      } catch (statError) {
        if (statError.code !== 'ENOENT') throw statError
      }
      const parent = dirname(ancestor)
      if (parent === ancestor) throw error
      missing.unshift(relative(parent, ancestor))
      ancestor = parent
    }
  }
}

function existingDirectory(fullPath) {
  let current = fullPath
  for (;;) {
    try {
      return lstatSync(current).isDirectory() ? current : dirname(current)
    } catch (error) {
      if (error.code !== 'ENOENT' || dirname(current) === current) throw error
      current = dirname(current)
    }
  }
}

function git(args, cwd, preserveEnvironment = false) {
  const env = { ...process.env }
  if (!preserveEnvironment) {
    for (const key of Object.keys(env)) {
      if (key.startsWith('GIT_')) delete env[key]
    }
    // Discovery diagnostics are parsed only in this controlled locale.
    env.LC_ALL = 'C'
    env.LANGUAGE = 'C'
  }
  return spawnSync('git', args, {
    cwd,
    env,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  })
}

function gitUnavailable(res) {
  return Boolean(res.error) || res.status === 128 || res.status == null
}

function gitRoot(cwd, preserveEnvironment = false) {
  const res = git(['rev-parse', '--show-toplevel'], cwd, preserveEnvironment)
  if (gitUnavailable(res) || res.status !== 0) return ''
  return String(res.stdout || '').trim()
}

/** Require reciprocal Git metadata and an exact worktree registry entry. */
export function isWorktreePath(fullPath) {
  if (!fullPath) return false
  try {
    const target = canonicalTarget(fullPath)
    const cwd = existingDirectory(target)
    const root = gitRoot(cwd)
    if (!root) return false
    const query = (args) => {
      const result = git(args, cwd)
      if (result.status !== 0 || result.error) throw new Error('Git metadata unavailable')
      return result.stdout.trim()
    }
    const top = realpathSync(root)
    // Git can skip malformed nested .git directories and discover an outer repo.
    for (let parent = cwd; parent !== top; parent = dirname(parent)) {
      if (dirname(parent) === parent) return false
      try {
        lstatSync(resolve(parent, '.git'))
        return false
      } catch (error) {
        if (error.code !== 'ENOENT') return false
      }
    }
    const gitDir = realpathSync(query(['rev-parse', '--absolute-git-dir']))
    const common = realpathSync(query(['rev-parse', '--path-format=absolute', '--git-common-dir']))
    if (gitDir === common || dirname(gitDir) !== resolve(common, 'worktrees')) return false
    const marker = resolve(top, '.git')
    if (!lstatSync(marker).isFile()) return false
    if (realpathSync(readFileSync(resolve(gitDir, 'gitdir'), 'utf8').trim()) !== marker) return false
    const pointer = readFileSync(marker, 'utf8').trim()
    if (!pointer.startsWith('gitdir: ') || realpathSync(resolve(top, pointer.slice(8))) !== gitDir) return false
    const registry = query(['worktree', 'list', '--porcelain', '-z'])
    return registry.split('\0\0').some((record) =>
      record.split('\0').some((field) => field === `worktree ${top}`))
  } catch {
    return false
  }
}

export function isEphemeralPath(fullPath) {
  const parts = fullPath.split(/[\\/]/).filter(Boolean)
  if (parts.some((part) => EPHEMERAL_DIR_NAMES.has(part))) return true
  const base = parts[parts.length - 1] || ''
  if (EPHEMERAL_BASENAMES.has(base)) return true
  return EPHEMERAL_SUFFIXES.some((suffix) => base.endsWith(suffix))
}

export function isProtectedScope(fullPath, cwd) {
  const root = gitRoot(cwd) || cwd
  const normalizedFull = String(fullPath || '').replace(/\\/g, '/')
  const normalizedRoot = String(root || '').replace(/\\/g, '/').replace(/\/+$/, '')

  let relative = normalizedFull
  if (normalizedRoot && normalizedFull.startsWith(normalizedRoot + '/')) {
    relative = normalizedFull.slice(normalizedRoot.length + 1)
  } else if (normalizedFull.startsWith('/')) {
    const marker = '/OmniMux/'
    const idx = normalizedFull.lastIndexOf(marker)
    if (idx !== -1) {
      relative = normalizedFull.slice(idx + marker.length)
    }
  }

  // 1. Protected directory prefix match
  if (PROTECTED_DIR_PREFIXES.some((prefix) => relative.startsWith(prefix) || relative.includes('/' + prefix))) {
    return true
  }

  // 2. Protected root files
  if (PROTECTED_ROOT_FILES.has(relative) || Array.from(PROTECTED_ROOT_FILES).some((f) => relative.endsWith('/' + f))) {
    return true
  }

  return false
}

export function isGitIgnored(fullPath, cwd) {
  const root = gitRoot(cwd) || cwd
  const res = git(['check-ignore', '-q', '--', fullPath], root)
  if (gitUnavailable(res)) return false
  return res.status === 0
}

/**
 * Confirm failed discovery is not hiding malformed or unreadable metadata.
 * @param {string} cwd Canonical existing target directory.
 * @returns {boolean} Whether every ancestor is free of Git markers.
 */
function hasNoGitMetadata(cwd) {
  for (let parent = cwd; ; parent = dirname(parent)) {
    try {
      lstatSync(resolve(parent, '.git'))
      return false
    } catch (error) {
      if (error.code !== 'ENOENT') return false
    }
    if (dirname(parent) === parent) return true
  }
}

/**
 * Classify Git state without treating a failed read as proof of tracking.
 * @param {string} fullPath Canonical target path.
 * @param {string} cwd Existing target directory.
 * @returns {'non-git-target'|'git-read-error'|'tracked-file'|'untracked-draft'}
 */
function gitFileState(fullPath, cwd) {
  const discovery = git(['rev-parse', '--show-toplevel'], cwd)
  if (discovery.error || discovery.status !== 0) {
    const notRepo = !discovery.error && discovery.status === 128 &&
      !String(discovery.stdout || '').trim() &&
      String(discovery.stderr || '').trim() === 'fatal: not a git repository (or any of the parent directories): .git'
    return notRepo && hasNoGitMetadata(cwd) ? 'non-git-target' : 'git-read-error'
  }
  const root = String(discovery.stdout || '').trim()
  if (!root) return 'git-read-error'
  const res = git(['ls-files', '--error-unmatch', '--', fullPath], root)
  if (res.error || ![0, 1].includes(res.status)) return 'git-read-error'
  return res.status === 0 ? 'tracked-file' : 'untracked-draft'
}

export function isGitTracked(fullPath, cwd) {
  const state = gitFileState(fullPath, cwd)
  // Keep the boolean API conservative for callers that cannot represent errors.
  return state === 'tracked-file' || state === 'git-read-error'
}

function toFullPath(filePath, cwd) {
  if (!filePath) return ''
  return filePath.startsWith('/') || isAbsolute(filePath) ? filePath : resolve(cwd, filePath)
}

export function getUnpushedCommits(cwd) {
  const root = gitRoot(cwd, true) || cwd
  const res = git(['rev-list', 'origin/main..HEAD'], root, true)
  if (gitUnavailable(res) || res.status !== 0) return []
  return String(res.stdout || '')
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
}

function stripQuotedSpans(command) {
  return String(command)
    .replace(/\\"/g, '')
    .replace(/\\'/g, '')
    .replace(/"(?:[^"\\]|\\.)*"/g, ' ')
    .replace(/'(?:[^'\\]|\\.)*'/g, ' ')
}

function commandSegments(command) {
  return stripQuotedSpans(command)
    .split(/\s*(?:&&|\|\||;|\n)\s*/)
    .map((s) => s.trim())
    .filter(Boolean)
}

function looksLikeGitInvocation(segment) {
  return /(?:^|\s)git(?:\s|$)/i.test(segment)
}

/**
 * Drop heredoc bodies before tokenising. A heredoc is data handed to a command,
 * not a command of its own: `cat > note.sh <<EOF ... mkdir -p docs/y ... EOF`
 * writes exactly one file, and treating the body as shell would deny writing a
 * script whose content merely mentions a protected path.
 * @param {string} command
 * @returns {string}
 */
function stripHeredocBodies(command) {
  const lines = String(command).split('\n')
  const kept = []
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i]
    kept.push(line)
    // `<<EOF`, `<<-EOF`, `<< 'EOF'`, `<<"EOF"` — an optional dash strips tabs.
    const marker = line.match(/<<-?\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)\1/)
    if (!marker) continue
    const delimiter = marker[2]
    const stripTabs = line.includes('<<-')
    for (i += 1; i < lines.length; i += 1) {
      const body = stripTabs ? lines[i].replace(/^\t+/, '') : lines[i]
      if (body.trim() === delimiter) {
        kept.push(lines[i])
        break
      }
    }
  }
  return kept.join('\n')
}

/**
 * Split a command into words and control operators, honouring quotes so that a
 * `>` inside `-m "fix a > b"` stays part of the message instead of becoming a
 * redirect. Only the operators this guard needs are recognised.
 * @param {string} command
 * @returns {string[]}
 */
function shellTokens(command) {
  const tokens = []
  let word = ''
  let quote = ''
  const flush = () => {
    if (word) {
      tokens.push(word)
      word = ''
    }
  }
  for (let i = 0; i < command.length; i += 1) {
    const ch = command[i]
    if (quote) {
      if (ch === quote) quote = ''
      else if (ch === '\\' && quote === '"' && i + 1 < command.length) {
        word += command[i + 1]
        i += 1
      } else word += ch
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      continue
    }
    if (ch === '\\' && i + 1 < command.length) {
      word += command[i + 1]
      i += 1
      continue
    }
    if (ch === '\n') {
      flush()
      tokens.push(';')
      continue
    }
    if (/\s/.test(ch)) {
      flush()
      continue
    }
    if (ch === '&' && command[i + 1] === '>') {
      flush()
      tokens.push('>')
      i += 1
      continue
    }
    if (ch === '>' && command[i + 1] === '>') {
      flush()
      tokens.push('>>')
      i += 1
      continue
    }
    if (ch === '>') {
      // A leading fd number (`2>`) belongs to the redirect, not to a word.
      if (/^\d+$/.test(word)) word = ''
      flush()
      tokens.push('>')
      continue
    }
    if (';&|'.includes(ch)) {
      flush()
      tokens.push(ch)
      continue
    }
    word += ch
  }
  flush()
  return tokens
}

/** Commands whose every non-flag argument is written. */
const WRITE_ALL_ARGS = new Set(['mkdir', 'touch', 'tee', 'truncate'])
/** Commands whose final non-flag argument is the destination. */
const WRITE_LAST_ARG = new Set(['cp', 'mv', 'install', 'ln', 'link', 'rsync'])

/**
 * Paths a shell command would create or overwrite in the repository.
 *
 * Deliberately pattern-based rather than a shell interpreter: it covers the
 * write forms an agent actually reaches for (redirects, tee, cp/mv/ln/mkdir/
 * touch/install/rsync, `dd of=`, `sed -i`). Work that hides the target behind a
 * variable, a subshell or a script file is out of reach here and is reported as
 * such in docs/ops/worktree-hook-guard-zh.md instead of being silently claimed.
 *
 * @param {string} command
 * @returns {string[]}
 */
export function bashWriteTargets(command) {
  if (!command || typeof command !== 'string') return []
  const targets = []
  let segment = []
  const flushSegment = () => {
    if (segment.length) collectSegmentWrites(segment, targets)
    segment = []
  }
  for (const token of shellTokens(stripHeredocBodies(command))) {
    if (token === '&&' || token === '||' || token === ';' || token === '|' || token === '&') {
      flushSegment()
      continue
    }
    segment.push(token)
  }
  flushSegment()
  return targets
}

/** @param {string[]} tokens @param {string[]} targets */
function collectSegmentWrites(tokens, targets) {
  // A nested shell re-parses its own script, so the script it is handed must be
  // read as a command: `bash -c "echo hi > docs/x.md"` writes docs/x.md.
  const commandName = tokens[0] ? tokens[0].split('/').pop() : ''
  if (['bash', 'sh', 'zsh', 'dash', 'ksh'].includes(commandName)) {
    // `-c` may arrive combined with other single-letter flags (`bash -lc`).
    const scriptIndex = tokens.findIndex((token) => /^-[a-zA-Z]*c[a-zA-Z]*$/.test(token))
    if (scriptIndex !== -1 && tokens[scriptIndex + 1]) {
      targets.push(...bashWriteTargets(tokens[scriptIndex + 1]))
    }
    return
  }

  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token !== '>' && token !== '>>') continue
    const next = tokens[i + 1]
    // `>&2` duplicates a descriptor; `2>&1` names no file.
    if (!next || next === '&' || /^&\d+$/.test(next) || /^\d+$/.test(next)) continue
    targets.push(next)
    i += 1
  }

  const args = tokens
    .slice(1)
    .filter((token) => token !== '>' && token !== '>>' && !/^-/.test(token))

  if (commandName === 'dd') {
    for (const arg of tokens.slice(1)) {
      if (arg.startsWith('of=')) targets.push(arg.slice(3))
    }
    return
  }
  if (commandName === 'sed') {
    // `sed -i` rewrites its input files in place. The script expression is the
    // only non-file argument, so everything left after dropping it is a target.
    const inPlace = tokens.slice(1).some((token) => token === '-i' || /^-[a-zA-Z]*i/.test(token))
    if (!inPlace) return
    const files = args.filter((token) => !/^s?[/|#,]/.test(token) && !/^[0-9,$]+[dp]?$/.test(token))
    targets.push(...files)
    return
  }
  if (WRITE_ALL_ARGS.has(commandName)) {
    targets.push(...args)
    return
  }
  if (WRITE_LAST_ARG.has(commandName) && args.length) {
    targets.push(args[args.length - 1])
  }
}

/**
 * Resolve one shell write target to the same verdict a `write` tool call would
 * receive. Returns null for targets that name no repository path.
 * @param {string} target
 * @param {string} cwd
 * @returns {{decision: string, reason: string, fullPath: string}|null}
 */
function classifyBashWriteTarget(target, cwd) {
  if (!target || target === '-' || target === '/dev/null' || target === '/dev/stdout') return null
  if (target.startsWith('&') || target.startsWith('$') || target.includes('$(')) return null
  const result = decideWrite({ filePath: target, cwd, toolName: 'write' })
  if (result.decision === 'allow') return null
  return { decision: result.decision, reason: result.reason, fullPath: result.fullPath || target }
}

/**
 * A checkout argument that names a file, not a ref. A bare branch name may
 * contain `/` (`feat/x`), so only a leading `.`/`/`/`*`, a trailing separator,
 * or a file-extension tail counts as a pathspec.
 * @param {string} token
 * @returns {boolean}
 */
function looksLikePathspec(token) {
  if (token === '.' || token === '..') return true
  if (token.startsWith('/') || token.startsWith('*') || token.endsWith('/')) return true
  return /\.(go|md|json|ya?ml|tsx?|jsx?|mjs|cjs|sh|css|scss|html|sql|toml|ini|env|txt|png|jpe?g|svg|lock|mod|sum)$/i.test(token)
}

/**
 * The ref a `git checkout` segment would switch to, or null when the segment
 * only touches the working tree (`git checkout -- <paths>`) or names nothing.
 * `git checkout <commit>` detaches HEAD, so an unresolvable name still counts.
 */
function checkoutSwitchTarget(segment) {
  const match = segment.match(/(?:^|\s)git\s[^&|;]*?\b(?:checkout|co)\s+([\s\S]*)$/i)
  if (!match) return null
  const tokens = match[1].trim().split(/\s+/).filter(Boolean)
  if (!tokens.length) return null
  let index = 0
  while (index < tokens.length && tokens[index].startsWith('-')) {
    const flag = tokens[index].slice(1)
    if ('bBcC'.includes(flag)) {
      const next = tokens[index + 1]
      return next && !next.startsWith('-') ? next : null
    }
    if (/[bBcC]/.test(flag)) return null
    if (flag === '-') return null
    index += 1
  }
  if (index >= tokens.length) return null
  const candidate = tokens[index]
  return looksLikePathspec(candidate) ? null : candidate
}

/** The branch a `git switch` segment would move to (null for `--detach`). */
function switchTarget(segment) {
  const match = segment.match(/(?:^|\s)git\s[^&|;]*?\b(?:switch|sw)\s+([\s\S]*)$/i)
  if (!match) return null
  const tokens = match[1].trim().split(/\s+/).filter(Boolean)
  let index = 0
  while (index < tokens.length && tokens[index].startsWith('-')) {
    const flag = tokens[index].slice(1)
    if ('cC'.includes(flag)) {
      const next = tokens[index + 1]
      return next && !next.startsWith('-') ? next : '-'
    }
    if (/[cC]/.test(flag)) return '-'
    if (flag === 'detach') return '-'
    index += 1
  }
  if (index >= tokens.length) return null
  return tokens[index]
}

/**
 * Whether the segment moves the HEAD of the checkout it runs in to another
 * branch. Every worktree keeps its own HEAD, so this is checked against the
 * repository the command actually runs in; only the primary checkout is
 * constrained (a linked worktree is where branch work belongs).
 */
export function isBranchSwitchCommand(command) {
  if (!command || typeof command !== 'string') return false
  return commandSegments(command).some((seg) => {
    if (!looksLikeGitInvocation(seg)) return false
    // Global flags (`git -c k=v checkout x`) sit between `git` and the verb.
    if (/(?:^|\s)git\s[^&|;]*?\b(?:checkout|co|switch|sw)(?=\s|$)/i.test(seg)) {
      const viaCheckout = checkoutSwitchTarget(seg)
      if (viaCheckout !== null) return true
      const viaSwitch = switchTarget(seg)
      if (viaSwitch !== null) return true
    }
    return false
  })
}

function readHeadBranch(gitDir) {
  try {
    const head = readFileSync(resolve(gitDir, 'HEAD'), 'utf8').trim()
    const match = head.match(/^ref:\s*refs\/heads\/(.+)$/)
    return match ? match[1] : ''
  } catch {
    return ''
  }
}

/**
 * Classify the checkout a command runs in. `primary: false` means a linked
 * worktree, where switching branches is the normal workflow.
 * @param {string} cwd
 * @returns {{gitDir: string, common: string, primary: boolean, branch: string}}
 */
function checkoutContext(cwd) {
  const query = (args) => {
    const res = git(args, cwd)
    if (res.error || res.status !== 0) return ''
    return String(res.stdout || '').trim()
  }
  const gitDir = query(['rev-parse', '--absolute-git-dir'])
  const common = query(['rev-parse', '--path-format=absolute', '--git-common-dir'])
  const primary = Boolean(gitDir) && Boolean(common) && canonicalTarget(gitDir) === canonicalTarget(common)
  return { gitDir, common, primary, branch: gitDir ? readHeadBranch(gitDir) : '' }
}

export function branchSwitchDenial({ branch, target }) {
  const override = 'ℹ️  仅当确实要在主检出切分支时，可在命令前加 `OMNIMUX_ALLOW_BRANCH_SWITCH=1` 一次性放行（不要写进配置或脚本）；日常改代码请走 worktree。'
  if (target === '-') {
    return [
      '🚫【OmniMux 仓库 Hook】禁止在主 checkout 上 `git switch --detach` 或新建分支：主 checkout 的 HEAD 必须始终指向 main！',
      '📌 核心防线原则：主 checkout 是只读镜像（clean mirror of origin/main），一切改动都在独立 worktree 里完成，分支切换一律在工作树内进行。',
      `👉 正确流程：bash 运行 ${WORKTREE_COMMAND} 创建并切入独立工作树，再在 worktree 内 git switch / 新建分支。`,
      override,
    ].join('\n')
  }
  if (branch && target === branch) {
    return [
      `🚫【OmniMux 仓库 Hook】主 checkout 已经位于 ${branch}，无需切换；且禁止在主 checkout 重建同名分支（HEAD 必须始终指向 main）。`,
      `👉 正确流程：bash 运行 ${WORKTREE_COMMAND}，在独立工作树内创建/切换分支。`,
      override,
    ].join('\n')
  }
  return [
    `🚫【OmniMux 仓库 Hook】禁止在主 checkout 切换分支（当前在 ${branch || 'detached HEAD'}，目标是 ${target}）！主 checkout 的 HEAD 必须始终指向 main。`,
    '📌 核心防线原则：主 checkout 只做 main 的干净镜像；一旦切走，未提交改动会被覆盖或带错分支，其他 Agent 的 worktree 基线也会随之漂移。',
    `👉 正确流程：bash 运行 ${WORKTREE_COMMAND} 创建并切入独立工作树，所有分支操作与改动都在 worktree 内完成。`,
    override,
  ].join('\n')
}

export function isDestructiveResetCommand(command) {
  if (!command || typeof command !== 'string') return false
  return commandSegments(command).some((seg) => {
    if (!looksLikeGitInvocation(seg)) return false
    if (/\breset\b/i.test(seg) && /(?:^|\s)--hard\b/.test(seg)) return true
    if (/\bcheckout\b/i.test(seg) && /(?:^|\s)-f\b/.test(seg)) return true
    if (/\brestore\b/i.test(seg) && /--source=origin\/(?:main|master)\b/.test(seg)) return true
    if (/\bclean\b/i.test(seg) && /(?:^|\s)-[a-z]*[fx]\b/i.test(seg)) return true
    return false
  })
}

export function decideBashCommand({ command, cwd }) {
  if (!command || typeof command !== 'string') return { decision: 'allow' }

  // The primary checkout mirrors origin/main: its HEAD may not leave main.
  // A linked worktree owns its own HEAD, so branch work is unrestricted there.
  // The override exists for the operator who really means it; it is deliberate,
  // visible in the command itself, and never silent.
  if (isBranchSwitchCommand(command) && process.env.OMNIMUX_ALLOW_BRANCH_SWITCH !== '1') {
    const context = checkoutContext(cwd)
    if (context.primary) {
      const segments = commandSegments(command)
      const target = segments.map((seg) => checkoutSwitchTarget(seg)).find((t) => t !== null) ||
        segments.map((seg) => switchTarget(seg)).find((t) => t !== null) ||
        '-'
      return {
        decision: 'deny',
        reason: 'primary-checkout-branch-pinned',
        branch: context.branch,
        target,
      }
    }
  }

  // A shell redirect or `cp`/`mv`/`ln` writes the same files an `edit`/`write`
  // call would, so it gets the same verdict. Without this the write guard is
  // advisory: one `cat > docs/x.md` walks around it.
  const writeTargets = bashWriteTargets(command)
    .map((target) => classifyBashWriteTarget(target, cwd))
    .filter(Boolean)
  if (writeTargets.length) {
    return {
      decision: 'deny',
      reason: writeTargets[0].reason,
      bashWriteTargets: writeTargets.map((entry) => entry.fullPath),
    }
  }

  if (!isDestructiveResetCommand(command)) return { decision: 'allow' }

  // Check if unpushed commits exist on current HEAD
  const unpushed = getUnpushedCommits(cwd)
  if (unpushed.length > 0) {
    return {
      decision: 'deny',
      reason: 'unpushed-commits-at-risk',
      unpushedCount: unpushed.length,
      unpushedCommits: unpushed,
    }
  }
  return { decision: 'allow', reason: 'clean-upstream' }
}

export function decideWrite({ filePath, cwd, toolName }) {
  // Normalize toolName to handle namespace e.g. "default_api:edit" -> "edit"
  const rawName = String(toolName || '').toLowerCase()
  const name = rawName.replace(/^.*:/, '')
  if (name !== 'edit' && name !== 'write') return { decision: 'allow' }

  let fullPath = toFullPath(String(filePath || '').trim(), cwd)
  if (!fullPath) return { decision: 'allow' }
  try {
    fullPath = canonicalTarget(fullPath)
    cwd = existingDirectory(fullPath)
  } catch {
    return { decision: 'deny', fullPath, reason: 'unresolved-target' }
  }

  // 1. Linked worktree directories are fully exempt
  if (isWorktreePath(fullPath)) {
    return { decision: 'allow', fullPath, reason: 'worktree-isolated' }
  }

  // 2. Ephemeral and derived paths (node_modules, dist, tmp, coverage, *.log)
  if (isEphemeralPath(fullPath)) {
    return { decision: 'allow', fullPath, reason: 'ephemeral' }
  }

  // 3. Paths matching .gitignore rules
  if (isGitIgnored(fullPath, cwd)) {
    return { decision: 'allow', fullPath, reason: 'gitignored' }
  }

  // Classification is not cross-workspace authorization; that remains external.
  const state = gitFileState(fullPath, cwd)
  if (state === 'non-git-target') {
    return { decision: 'allow', fullPath, reason: state }
  }
  if (state === 'tracked-file' || state === 'git-read-error') {
    return { decision: 'deny', fullPath, reason: state }
  }

  // 5. Untracked additions inside protected core directories are refused too
  if (isProtectedScope(fullPath, cwd)) {
    return { decision: 'deny', fullPath, reason: 'untracked-protected-scope' }
  }

  // 6. Remaining untracked local drafts (workbench notes, throwaway probes) are allowed
  return { decision: 'allow', fullPath, reason: 'untracked-draft' }
}

function decisionJson(hookEventName, decision, reason, extra = {}) {
  const output = {
    hookEventName,
    permissionDecision: decision,
  }
  if (decision === 'deny') {
    if (reason === 'unpushed-commits-at-risk') {
      output.permissionDecisionReason = [
        `🚫【OmniMux 仓库 Hook】拦截破坏性重置：检测到当前本地分支存在尚未推送到远端 origin/main 的提交（共 ${extra.unpushedCount || 0} 个）！`,
        '📌 事故防范守则：执行 git reset --hard 会导致这些未同步的本地工作成果被永久抹去。',
        '👉 正确流程：',
        '  1. 若成果有效：切到工作分支推送远端（./scripts/ops/worktree.sh ship <task> 或 git push 后提 PR）；',
        '  2. 若确需重置：先打备份标签（git tag backup/safety-$(date +%s)）后再安全处理。',
      ].join('\n')
    } else if (reason === 'primary-checkout-branch-pinned') {
      output.permissionDecisionReason = branchSwitchDenial(extra)
    } else if (reason === 'git-read-error') {
      output.permissionDecisionReason = '🚫【OmniMux 仓库 Hook】Git 状态读取失败，无法确认目标的版本管理状态；保守拒绝写入。请检查 Git 可用性、仓库元数据与读取权限，不要绕过门禁。'
    } else if (reason === 'unresolved-target') {
      output.permissionDecisionReason = '🚫【OmniMux 仓库 Hook】目标路径无法安全解析；保守拒绝写入。请检查路径读取权限、符号链接及父目录。'
    } else if (reason === 'untracked-protected-scope') {
      output.permissionDecisionReason = [
        '🚫【OmniMux 仓库 Hook】严禁在主 checkout 的 scripts/、deploy/、docs/、.githooks/、.agents/、.github/ 等核心目录下新建任何源码或配置文件！',
        '📌 核心防线原则：在主 checkout 直接创建未跟踪文件会污染主干工作区，并引发构建衍生与多 Agent 冲突覆盖。',
        `👉 正确流程：请先调用 bash 运行: ${WORKTREE_COMMAND} 创建并切入独立 Worktree 工作区！`,
        'ℹ️  豁免范围：经 Git 元数据与注册表核实的 linked worktree、.gitignore 规则文件、临时缓存目录（tmp/、dist/、node_modules/、*.log）。',
      ].join('\n')
    } else {
      const viaBash = Array.isArray(extra.bashWriteTargets) && extra.bashWriteTargets.length > 0
      output.permissionDecisionReason = [
        viaBash
          ? '🚫【OmniMux 仓库 Hook】这条 bash 命令会写入主 checkout 中受保护或已跟踪的文件，已被拦截（与 edit/write 同一判定）。'
          : '🚫【OmniMux 仓库 Hook】严禁在主 checkout 直接修改任何已加入版本管理（Git Tracked）的文件！',
        viaBash ? `📄 命中目标：${extra.bashWriteTargets.join('、')}` : null,
        '📌 核心防线原则：AGENTS.md「主 checkout 保持干净」——版本管理的文件一旦在主目录被修改，将面临未经审核的脏提交，或在同步拉取时被覆盖/丢弃。',
        `👉 正确流程：请先调用 bash 运行: ${WORKTREE_COMMAND} 创建并切入独立 Worktree 工作区！`,
        'ℹ️  豁免范围：独立 Worktree 目录、gitignore 规则文件、临时缓存（node_modules、dist、tmp、*.log）与非受保护草稿文件。',
      ].filter(Boolean).join('\n')
    }
  }
  return { hookSpecificOutput: output }
}

function handle(rawInput) {
  const input = JSON.parse(rawInput || '{}')
  const hookEventName = input.hook_event_name || 'PreToolUse'
  const rawTool = String(input.tool_name || '').toLowerCase()
  const toolName = rawTool.replace(/^.*:/, '')
  const toolInput = input.tool_input || {}
  // Anchor on the guard's own checkout whenever the reported cwd is not absolute:
  // the target repo of a hook payload is the repository this guard lives in.
  const reportedCwd = String(input.cwd || '')
  let cwd = scriptRepo
  if (isAbsolute(reportedCwd)) {
    try {
      cwd = canonicalTarget(reportedCwd)
    } catch {
      cwd = scriptRepo
    }
  }

  if (toolName === 'bash') {
    const command = String(toolInput.command || toolInput.cmd || '').trim()
    const result = decideBashCommand({ command, cwd })
    return decisionJson(hookEventName, result.decision, result.reason, result)
  }

  const filePath = String(toolInput.file_path || toolInput.path || '').trim()
  const result = decideWrite({ filePath, cwd, toolName })
  return decisionJson(hookEventName, result.decision, result.reason, result)
}

function main() {
  let rawInput = ''
  process.stdin.setEncoding('utf8')
  process.stdin.on('data', (chunk) => {
    rawInput += chunk
  })
  process.stdin.on('end', () => {
    try {
      process.stdout.write(JSON.stringify(handle(rawInput)) + '\n')
    } catch (err) {
      process.stderr.write(`[guard-worktree error] ${err.message}\n`)
      process.stdout.write(
        JSON.stringify({
          hookSpecificOutput: {
            hookEventName: 'PreToolUse',
            permissionDecision: 'allow',
          },
        }) + '\n',
      )
    }
  })
}

const isMain = Boolean(process.argv[1]) && import.meta.url === pathToFileURL(resolve(process.argv[1])).href

if (isMain) main()
