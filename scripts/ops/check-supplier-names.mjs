#!/usr/bin/env node
// Refuses channel-supplier names on surfaces a user can actually see.
//
// Policy (product owner, 2026-11-13): backend and admin naming may mention the
// upstream a channel buys from, but nothing a customer reads may. That covers
// model ids, the public price list, the landing page, the published API docs and
// the skill packages shipped with the CLI.
//
// Usage:
//   node scripts/ops/check-supplier-names.mjs            # scan the repo
//   node scripts/ops/check-supplier-names.mjs --list     # also list scanned roots
//
// Exit 0 clean, 1 findings, 2 usage error.

import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join, relative, extname } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = process.cwd()

/** Where a customer reads text: shipped docs, public pages, published contracts. */
const SCANNED_ROOTS = [
  'cli/skill',
  'web/src/features/pricing',
  'web/src/features/omnimux-landing',
  'docs/openapi',
  'docs/plugin-api',
]

/** Single files that publish model identity to clients. */
const SCANNED_FILES = ['modelcontracts/contracts.json']

/**
 * Upstream resellers we buy from. Model vendor brands (volcengine, jimeng,
 * kling, vertex, openai, gemini, anthropic, deepseek) are deliberately absent:
 * those name the model itself, are public knowledge and appear on the model page.
 */
const FORBIDDEN = [
  'autodl',
  'weijin',
  'pidoi',
  'apikeyfun',
  'evolink',
  'apimart',
  'goeasy',
  'runninghub',
  'gxgen',
]

const MATCHER = new RegExp(FORBIDDEN.join('|'), 'i')
const TEXT_EXTENSIONS = new Set([
  '.md',
  '.mdx',
  '.json',
  '.yaml',
  '.yml',
  '.ts',
  '.tsx',
  '.js',
  '.jsx',
  '.mjs',
  '.txt',
  '.html',
])

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === '.git') continue
      yield* walk(path)
    } else if (entry.isFile() && TEXT_EXTENSIONS.has(extname(entry.name).toLowerCase())) {
      yield path
    }
  }
}

function scanFile(path) {
  const findings = []
  let text
  try {
    text = readFileSync(path, 'utf8')
  } catch {
    return findings
  }
  text.split('\n').forEach((line, index) => {
    const match = line.match(MATCHER)
    if (match) findings.push({ file: relative(ROOT, path), line: index + 1, name: match[0] })
  })
  return findings
}

function main(argv) {
  if (argv.includes('--help') || argv.includes('-h')) {
    console.log('usage: node scripts/ops/check-supplier-names.mjs [--list]')
    return 0
  }
  if (argv.includes('--list')) {
    console.log('scanned roots:', SCANNED_ROOTS.join(', '))
    console.log('scanned files:', SCANNED_FILES.join(', '))
  }

  const findings = []
  let scanned = 0
  for (const root of SCANNED_ROOTS) {
    const abs = join(ROOT, root)
    if (!existsSync(abs)) continue
    const stat = statSync(abs)
    const files = stat.isDirectory() ? walk(abs) : [abs]
    for (const file of files) {
      scanned += 1
      findings.push(...scanFile(file))
    }
  }
  for (const file of SCANNED_FILES) {
    const abs = join(ROOT, file)
    if (!existsSync(abs)) continue
    scanned += 1
    findings.push(...scanFile(abs))
  }

  if (findings.length === 0) {
    console.log(`supplier-name gate: ${scanned} files scanned, no findings`)
    return 0
  }
  console.error(`supplier-name gate: ${findings.length} finding(s) on user-visible surfaces`)
  for (const f of findings) console.error(`  ${f.file}:${f.line}  ${f.name}`)
  console.error('\nRule: a customer must never read which upstream a channel buys from.')
  console.error('Rename to a neutral token, or move the detail into the private operations repository.')
  return 1
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  process.exit(main(process.argv.slice(2)))
}
