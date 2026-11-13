#!/usr/bin/env node
/**
 * scripts/ops/guard-ui-design.mjs
 * dsh-hooks-plugin PreToolUse Hard Gate for OmniMux UI design rules.
 *
 * Contract: DESIGN.md §4/§7 and docs/ops/ui-design-hook-guard-zh.md.
 * Only the 严禁 items that stay clean against the current tree are enabled;
 * the admission measurement for every candidate rule lives in that document.
 */

import { readFileSync } from 'node:fs'
import { dirname, isAbsolute, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { inspectUICode, isTargetUIFile, toRepoRelative } from './guard-ui-design-rules.mjs'

/** The checkout this guard ships with; payload cwd resolution falls back to it. */
const scriptRepo = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..')

const RULE_TITLES = {
  VIOLET_CTA: '紫色 CTA（§7 禁忌 1）',
  NEON_GREEN: '荧光绿（§7 禁忌 2）',
  MEDIA_CARD_WHITEWASH: '暗房卡被漂白（§7 禁忌 4）',
  OFF_TYPE_SCALE: '字阶越界（§3.2）',
  NATIVE_CONTROL: '设计系统层裸用原生控件（§4）',
  DARK_BLUE_TINT: '深色偏蓝杂色（§7 禁忌 7）',
}

function formatDenyReason(violations, relativePath) {
  const lines = [
    `🚫【OmniMux UI 设计规范硬门禁】${relativePath} 命中 ${violations.length} 条 DESIGN.md 铁律，写入被物理打断。`,
    '',
  ]
  for (const v of violations) {
    const title = RULE_TITLES[v.rule] || v.rule
    lines.push(`  · 第 ${v.line} 行 [${title}] ${v.message}`)
  }
  lines.push(
    '',
    '📌 设计规范真源：DESIGN.md（§3.2 字阶表 / §4 组件规范 / §7 铁律禁忌）。',
    '👉 个别确需特化的行，在同行加注释 `// exempt-ui <原因>` 即可豁免。',
  )
  return lines.join('\n')
}

export function handle(rawInput) {
  const input = JSON.parse(rawInput || '{}')
  const hookEventName = input.hook_event_name || 'PreToolUse'
  const rawTool = String(input.tool_name || '').toLowerCase()
  const toolName = rawTool.replace(/^.*:/, '')
  const toolInput = input.tool_input || {}

  // 1. Only writes and edits carry UI code.
  if (toolName !== 'write' && toolName !== 'edit') {
    return { hookSpecificOutput: { hookEventName, permissionDecision: 'allow' } }
  }

  // 2. Resolve the target. A relative or unusable cwd falls back to this checkout.
  const rawFilePath = String(toolInput.file_path || toolInput.path || toolInput.filePath || '').trim()
  if (!rawFilePath) {
    return { hookSpecificOutput: { hookEventName, permissionDecision: 'allow' } }
  }
  const reportedCwd = String(input.cwd || '')
  const cwd = isAbsolute(reportedCwd) ? reportedCwd : scriptRepo
  const fullPath = isAbsolute(rawFilePath) ? rawFilePath : resolve(cwd, rawFilePath)
  const relativePath = toRepoRelative(fullPath, cwd)

  // 3. Scope filter: only product UI files are inspected.
  if (!isTargetUIFile(relativePath)) {
    return { hookSpecificOutput: { hookEventName, permissionDecision: 'allow' } }
  }

  // 4. Inspect the payload: the whole file for `write`, the edited span for `edit`.
  const codeToCheck = toolName === 'write'
    ? String(toolInput.content || '')
    : String(toolInput.new_string ?? toolInput.newText ?? '')

  const violations = inspectUICode(codeToCheck, relativePath)
  if (violations.length === 0) {
    return { hookSpecificOutput: { hookEventName, permissionDecision: 'allow' } }
  }

  // Report the real line number when the edited span can be located in the file.
  if (toolName === 'edit') {
    try {
      const oldString = String(toolInput.old_string || '')
      if (oldString) {
        const fileContent = readFileSync(fullPath, 'utf8')
        const index = fileContent.indexOf(oldString)
        const offset = index === -1 ? 0 : fileContent.slice(0, index).split('\n').length - 1
        for (const violation of violations) violation.line += offset
      }
    } catch {
      /* Line offsets are cosmetic; keep the payload-relative numbers. */
    }
  }

  const reason = formatDenyReason(violations, relativePath)
  return {
    hookSpecificOutput: {
      hookEventName,
      permissionDecision: 'deny',
      permissionDecisionReason: reason,
    },
  }
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
      process.stderr.write(`[guard-ui-design error] ${err.message}\n`)
      process.stdout.write(
        JSON.stringify({
          hookSpecificOutput: { hookEventName: 'PreToolUse', permissionDecision: 'allow' },
        }) + '\n',
      )
    }
  })
}

const isMain = Boolean(process.argv[1]) && import.meta.url === pathToFileURL(resolve(process.argv[1])).href

if (isMain) main()
