#!/usr/bin/env node
/**
 * scripts/ops/guard-ui-design.test.mjs
 * Behavioural tests for scripts/ops/guard-ui-design.mjs.
 *
 * Run: node scripts/ops/guard-ui-design.test.mjs
 * The last suite replays the rules against every current UI file: all of them
 * must pass, which is how the enabled rule set stays honest about false positives.
 */

import { spawnSync } from 'node:child_process'
import { globSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { handle } from './guard-ui-design.mjs'
import { TYPE_SCALE } from './guard-ui-design-rules.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const guard = resolve(here, 'guard-ui-design.mjs')
const repo = resolve(here, '..', '..')

/** Decide in-process; one spawn would cost more than the whole replay. */
function decide(tool, filePath, toolInput, cwd = repo) {
  return handle(JSON.stringify({
    hook_event_name: 'PreToolUse',
    tool_name: tool,
    cwd,
    tool_input: { file_path: filePath, ...toolInput },
  })).hookSpecificOutput.permissionDecision
}

const CTA_VIOLET = 'export const A = () => <button className="bg-violet-600 text-white">Go</button>'
const CTA_TOKENS = 'export const B = () => <button className="bg-primary text-primary-foreground">Go</button>'
const PILL_VIOLET = 'export const C = () => <span className="bg-violet-500/15 text-violet-700">tier</span>'
const NEON = 'export const D = () => <div className="bg-[#C8F135]">x</div>'
const OFF_SCALE = 'export const E = () => <h1 className="text-[42px] font-bold">x</h1>'
const ON_SCALE = 'export const F = () => <h1 className="text-[44px] font-bold">x</h1>'
const EXEMPT = 'export const G = () => <h1 className="text-[42px]">x</h1> // exempt-ui hero optical size'
const WHITE_WASH = 'export const H = () => <div className="bg-white p-2"><video src="a.mp4" /></div>'
const NATIVE_SELECT = 'export const I = () => <select><option>a</option></select>'

const CSS_OFF = '.a { font-size: 12.5px; }\n.b { font-weight: 550; }\n.c { border-radius: 12px; }\n.d { gap: 7px; }\n.e { box-shadow: 0 10px 30px rgba(0,0,0,.2); }'
const CSS_OK = '.a { font-size: 16px; }\n.b { font-weight: 600; }\n.c { border-radius: 16px; }\n.d { gap: 8px; }\n.e { box-shadow: var(--shadow-md); }\n.f { font-size: clamp(30px, 3.6vw, 44px); }\n.g { border-radius: 999px; }\n.h { gap: 0; }'

const cases = [
  ['public CSS off-scale size/weight/radius/gap/shadow', 'write', 'web/src/features/omnimux-landing/x.css', { content: CSS_OFF }, 'deny'],
  ['public CSS on-scale values', 'write', 'web/src/features/omnimux-landing/x.css', { content: CSS_OK }, 'allow'],
  ['CSS outside public surfaces is tolerated', 'write', 'web/src/features/usage-logs/x.css', { content: CSS_OFF }, 'allow'],
  ['violet solid CTA fill', 'write', 'web/src/features/x/components/a.tsx', { content: CTA_VIOLET }, 'deny'],
  ['violet gradient CTA', 'write', 'web/src/features/x/components/a.tsx', { content: 'export const A = () => <button className="bg-gradient-to-r from-violet-600 to-purple-600">Go</button>' }, 'deny'],
  ['violet gradient on a CSS button selector', 'write', 'web/src/features/omnimux-landing/desktop/x.css', { content: '.omc .btn-primary {background:linear-gradient(180deg,#7F68F5,#6D54EB);color:#fff}' }, 'deny'],
  ['violet token on a CSS send key', 'write', 'web/src/features/x/x.css', { content: '.c-send {\n  background: var(--violet);\n}' }, 'deny'],
  ['violet hover on a CSS button selector', 'write', 'web/src/features/x/x.css', { content: '.cta:hover { background-color: rgb(121 97 242); }' }, 'deny'],
  ['paper CTA in CSS', 'write', 'web/src/features/omnimux-landing/desktop/x.css', { content: '.omc .btn-primary {background:var(--fg);color:var(--ink)}' }, 'allow'],
  ['translucent violet wash on a CSS button is not a fill', 'write', 'web/src/features/x/x.css', { content: '.lang button.on {background:rgba(121,97,242,.12)}\n.skill:hover {background:var(--violet-tint)}' }, 'allow'],
  ['solid violet on a non-button CSS selector', 'write', 'web/src/features/x/x.css', { content: '.chart i:last-child {background:linear-gradient(180deg,#AB9CFF,#7961F2)}' }, 'allow'],
  ['commented CSS violet CTA is ignored', 'write', 'web/src/features/x/x.css', { content: '/* .btn-primary {background:#7961F2} */\n.a {color:#fff}' }, 'allow'],
  ['§2.2 whitelisted CSS violet send key with exempt-ui', 'write', 'web/src/features/x/x.css', { content: '.c-send {background:var(--violet-soft)} /* exempt-ui DESIGN.md §2.2 */' }, 'allow'],
  ['token-backed primary CTA', 'write', 'web/src/features/x/components/a.tsx', { content: CTA_TOKENS }, 'allow'],
  ['translucent violet badge is not a CTA fill', 'write', 'web/src/features/x/components/a.tsx', { content: PILL_VIOLET }, 'allow'],
  ['neon green', 'write', 'web/src/features/x/components/a.tsx', { content: NEON }, 'deny'],
  ['off-scale size on a public surface', 'write', 'web/src/features/omnimux-landing/components/hero.tsx', { content: OFF_SCALE }, 'deny'],
  ['on-scale size on a public surface', 'write', 'web/src/features/omnimux-landing/components/hero.tsx', { content: ON_SCALE }, 'allow'],
  ['off-scale size outside public surfaces is tolerated', 'write', 'web/src/features/usage-logs/components/a.tsx', { content: OFF_SCALE }, 'allow'],
  ['exempt-ui marker', 'write', 'web/src/features/omnimux-landing/components/hero.tsx', { content: EXEMPT }, 'allow'],
  ['media card washed to white', 'write', 'web/src/features/omnimux-landing/components/commerce/media-poster.tsx', { content: WHITE_WASH }, 'deny'],
  ['media card kept dark', 'write', 'web/src/features/omnimux-landing/components/commerce/media-poster.tsx', { content: 'export const H = () => <div className="bg-[#11141D] p-2"><video src="a.mp4" /></div>' }, 'allow'],
  ['native select inside the design system layer', 'write', 'web/src/components/ui/selectx.tsx', { content: NATIVE_SELECT }, 'deny'],
  ['styled select outside the design system layer', 'write', 'web/src/features/x/components/a.tsx', { content: NATIVE_SELECT }, 'allow'],
  ['legacy cold blue tint #151A26 in TSX', 'write', 'web/src/features/x/components/a.tsx', { content: 'export const A = () => <div className="bg-[#151A26]">x</div>' }, 'deny'],
  ['legacy cold blue tint #14161C in CSS', 'write', 'web/src/features/influencer/x.css', { content: '.a { background: #14161c; }' }, 'deny'],
  ['neutral dark charcoal is allowed', 'write', 'web/src/features/influencer/x.css', { content: '.a { background: #141416; }' }, 'allow'],
  ['test files are out of scope', 'write', 'web/src/features/x/components/a.test.tsx', { content: CTA_VIOLET }, 'allow'],
  ['non-UI files are out of scope', 'write', resolve(repo, 'web/src/lib/api/errors.ts'), { content: CTA_VIOLET }, 'allow'],
  ['paths from another checkout are out of scope', 'write', '/elsewhere/other-app/web/src/features/x/components/a.tsx', { content: CTA_VIOLET }, 'allow'],
  ['chrome (landing header) is outside the type scale', 'write', 'web/src/features/omnimux-landing/components/landing-header.tsx', { content: 'export const N = () => <nav className="text-[15px]">x</nav>' }, 'allow'],
  ['backend files are out of scope', 'write', 'controller/x.go', { content: CTA_VIOLET }, 'allow'],
  ['relative cwd still resolves the file', 'write', 'web/src/features/omnimux-landing/components/hero.tsx', { content: OFF_SCALE }, 'deny', '.'],
  ['edit payload uses new_string', 'edit', 'web/src/features/omnimux-landing/components/hero.tsx', { new_string: OFF_SCALE, old_string: '' }, 'deny'],
  ['bash payloads pass', 'bash', '', { command: 'echo hi' }, 'allow'],
]

let failed = 0
for (const [name, tool, filePath, input, expected, cwd] of cases) {
  const decision = decide(tool, filePath, input, cwd ?? repo)
  const ok = decision === expected
  if (!ok) failed += 1
  console.log(`${ok ? 'ok  ' : 'FAIL'}  ${name}: ${decision}${ok ? '' : ` (want ${expected})`}`)
}

// Explicit protocol check: the hook contract must survive a real process boundary.
const protocol = spawnSync('node', [guard], {
  input: JSON.stringify({
    hook_event_name: 'PreToolUse',
    tool_name: 'write',
    cwd: repo,
    tool_input: { file_path: 'web/src/features/omnimux-landing/components/hero.tsx', content: OFF_SCALE },
  }),
  encoding: 'utf8',
  cwd: repo,
})
let protocolOk = false
try {
  const out = JSON.parse(protocol.stdout).hookSpecificOutput
  protocolOk = out.hookEventName === 'PreToolUse' && out.permissionDecision === 'deny'
} catch {
  protocolOk = false
}
if (!protocolOk) failed += 1
console.log(`${protocolOk ? 'ok  ' : 'FAIL'}  stdin/stdout hook protocol`)

// Tree replay: every existing UI file must pass the enabled rule set.
const treeFiles = globSync('web/src/**/*.{ts,tsx,css}', { cwd: repo })
  .filter((p) => !/\.(test|spec|stories)\./.test(p))
let treeViolations = 0
for (const rel of treeFiles) {
  let content
  try {
    content = readFileSync(join(repo, rel), 'utf8')
  } catch {
    continue
  }
  if (decide('write', rel, { content }) === 'deny') {
    treeViolations += 1
    if (treeViolations <= 5) console.log(`FAIL  tree replay denies existing file: ${rel}`)
  }
}
if (treeViolations > 5) console.log(`FAIL  …and ${treeViolations - 5} more`)

// Spec/gate sync: DESIGN.md §3.2 publishes the public-surface whitelist this rule
// enforces. If the two drift, the spec becomes decoration and the rule starts
// rejecting sizes the spec blesses (or waving through ones it bans).
const specWhitelist = (() => {
  try {
    const design = readFileSync(join(repo, 'DESIGN.md'), 'utf8')
    const block = design.match(/公开面字号白名单[\s\S]{0,240}?```\n([\s\S]*?)```/)
    if (!block) return null
    return block[1]
      .split(',')
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isFinite(value))
  } catch {
    return null
  }
})()
const specSynced =
  specWhitelist !== null &&
  specWhitelist.length === TYPE_SCALE.length &&
  specWhitelist.every((value, index) => value === TYPE_SCALE[index])
if (!specSynced) failed += 1
console.log(
  `${specSynced ? 'ok  ' : 'FAIL'}  DESIGN.md §3.2 whitelist matches TYPE_SCALE` +
    (specSynced
      ? ''
      : ` — spec=${JSON.stringify(specWhitelist)} rule=${JSON.stringify(TYPE_SCALE)}`)
)


// Token integrity: every var(--omx-*)/var(--shadow-*) referenced by the public
// marketing CSS must have a definition. A missing definition makes the property
// invalid, so the value silently falls back to initial — which is how an
// undefined --shadow-md turned every tokenised shadow into none.
const cssFiles = globSync("web/src/features/{omnimux-landing,inspiration}/**/*.css", { cwd: repo })
const usedTokens = new Set(), definedTokens = new Set()
for (const rel of cssFiles) {
  const src = readFileSync(join(repo, rel), "utf8").replace(/\/\*[\s\S]*?\*\//g, "")
  for (const m of src.matchAll(/var\((--[\w-]+)/g)) usedTokens.add(m[1])
  for (const m of src.matchAll(/^\s*(--[\w-]+)\s*:/gm)) definedTokens.add(m[1])
}
const undefinedTokens = [...usedTokens].filter(
  (v) => !definedTokens.has(v) && (v.startsWith("--omx-") || v.startsWith("--shadow-"))
)
const tokensOk = undefinedTokens.length === 0
if (!tokensOk) failed += 1
console.log(
  `${tokensOk ? "ok  " : "FAIL"}  public CSS tokens are all defined` +
    (tokensOk ? "" : ` — undefined: ${undefinedTokens.join(", ")}`)
)

console.log(`\nbehaviour: ${cases.length + 3 - failed}/${cases.length + 3} passed`)

console.log(`tree replay: ${treeFiles.length} files, ${treeViolations} denials`)
process.exit(failed === 0 && treeViolations === 0 ? 0 : 1)
