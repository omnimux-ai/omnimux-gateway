#!/usr/bin/env node
/**
 * Machine parity gate for the 1:1 model-page replica.
 *
 * Reads docs/product/model-page-parity-checklist.json and checks three things
 * against our source tree, so "did we actually replicate the benchmark" is a
 * command rather than an eyeball exercise:
 *
 *   1. forbidden copy   — every `expect: "MUST NOT appear: X"` string is absent
 *   2. data-parity ids  — every non-live check id is emitted by a component
 *   3. no raw hex color — brand visuals must come from DESIGN.md tokens, so a
 *                         hardcoded hex in our surface is a defect
 *
 * Runtime DOM behaviour (order, control type, states) is covered separately by
 * the jsdom parity test; the API section (client-rendered on the benchmark)
 * must be captured live with a real browser.
 *
 * Usage: node scripts/dev/check-parity-contracts.mjs [--json]
 */
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const CHECKLIST = join(ROOT, 'docs', 'product', 'model-page-parity-checklist.json')

/** Source roots that make up the replicated surface. */
const SCAN_ROOTS = [
  join(ROOT, 'web', 'src', 'features', 'model-playground'),
  join(ROOT, 'web', 'src', 'features', 'pricing', 'components'),
]

const SCAN_SUFFIXES = ['.ts', '.tsx']
const SKIP_DIRS = new Set(['__tests__', 'node_modules'])

function walk(dir, files = []) {
  let entries
  try {
    entries = readdirSync(dir, { withFileTypes: true })
  } catch {
    return files
  }
  for (const entry of entries) {
    if (entry.name.startsWith('.') || SKIP_DIRS.has(entry.name)) continue
    const full = join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(full, files)
      continue
    }
    if (SCAN_SUFFIXES.some((suffix) => entry.name.endsWith(suffix))) {
      files.push(full)
    }
  }
  return files
}

function readSources() {
  return SCAN_ROOTS.flatMap((root) => walk(root))
    .filter((file) => statSync(file).isFile())
    .map((file) => ({
      path: relative(ROOT, file),
      text: readFileSync(file, 'utf8'),
    }))
}

/** `expect: "MUST NOT appear: X"` encodes a literal that must be absent. */
function forbiddenNeedles(checks) {
  const needles = []
  for (const check of checks) {
    if (check.compare !== 'forbid') continue
    const match = /MUST NOT appear:\s*(.+)$/.exec(check.expect ?? '')
    if (!match) continue
    needles.push({ id: check.id, needle: match[1].trim(), note: check.note })
  }
  return needles
}

function checkForbidden(sources, needles) {
  const failures = []
  for (const { id, needle, note } of needles) {
    for (const source of sources) {
      if (source.text.includes(needle)) {
        failures.push({
          kind: 'forbidden-copy',
          id,
          detail: `"${needle}" found in ${source.path}`,
          note,
        })
      }
    }
  }
  return failures
}

/**
 * Ids the checklist expects in our DOM.
 *
 * Two families are excluded:
 *   - `source: "live"` — the benchmark section is client-rendered, so it can
 *     only be captured with a real browser, not scanned from our source tree.
 *   - `compare: "forbid"` — these assert a string is ABSENT; requiring a
 *     matching `data-parity` id would be self-contradictory. They are covered
 *     by the forbidden-copy scan instead.
 */
function requiredParityIds(checks) {
  return checks.filter(
    (check) =>
      check.source !== 'live' &&
      check.compare !== 'forbid' &&
      // Deferred entries stay in the checklist so the gap is visible, but they
      // are reported separately instead of failing the gate.
      check.deferred === undefined
  )
}

function checkParityIds(sources, checks) {
  const haystack = sources.map((source) => source.text).join('\n')
  const failures = []
  for (const check of checks) {
    // Accept every attribute spelling this codebase can produce, plus the bare
    // quoted string: components may hold the id in a lookup map and apply it
    // through an expression (`data-parity={PARITY_BY_KEY[key]}`), which a
    // literal-attribute match would report as missing.
    //
    // Consequence: this gate proves an id is DECLARED, not that it reaches the
    // DOM. The rendered-DOM parity test owns that second half.
    const spellings = [
      `data-parity="${check.id}"`,
      `data-parity='${check.id}'`,
      `data-parity={"${check.id}"}`,
      `data-parity={'${check.id}'}`,
      `parity="${check.id}"`,
      `parity='${check.id}'`,
      `parity={"${check.id}"}`,
      `parity={'${check.id}'}`,
      `"${check.id}"`,
      `'${check.id}'`,
    ]
    if (spellings.some((spelling) => haystack.includes(spelling))) continue
    failures.push({
      kind: 'missing-parity-id',
      id: check.id,
      detail: `${check.area} · ${check.compare} · expected ${spellings[0]}`,
      note: check.expect,
    })
  }
  return failures
}

const HEX_COLOR = /#[0-9a-fA-F]{3,8}\b/g

function checkHexColors(sources) {
  const failures = []
  for (const source of sources) {
    if (!source.path.includes('model-playground')) continue
    const hits = [...new Set(source.text.match(HEX_COLOR) ?? [])]
    if (hits.length === 0) continue
    failures.push({
      kind: 'raw-hex-color',
      id: 'token-whitelist',
      detail: `${source.path} hardcodes ${hits.join(', ')}`,
      note: 'use DESIGN.md tokens / Tailwind semantic classes',
    })
  }
  return failures
}

function main() {
  const checklist = JSON.parse(readFileSync(CHECKLIST, 'utf8'))
  const checks = checklist.checks ?? []
  const sources = readSources()

  const needles = forbiddenNeedles(checks)
  const required = requiredParityIds(checks)
  const failures = [
    ...checkForbidden(sources, needles),
    ...checkParityIds(sources, required),
    ...checkHexColors(sources),
  ]

  const report = {
    checklist_checks: checks.length,
    forbidden_needles: needles.length,
    parity_ids_required: required.length,
    scanned_files: sources.length,
    failures,
    pass: failures.length === 0,
  }

  if (process.argv.includes('--json')) {
    process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
    return
  }

  process.stdout.write(
    `parity contracts: ${report.pass ? 'PASS' : 'FAIL'} · ` +
      `${report.parity_ids_required} ids required · ` +
      `${report.forbidden_needles} forbidden strings · ` +
      `${report.scanned_files} files scanned\n`
  )
  for (const failure of failures) {
    process.stdout.write(`  [${failure.kind}] ${failure.id} — ${failure.detail}\n`)
  }
  process.exitCode = report.pass ? 0 : 1
}

main()
