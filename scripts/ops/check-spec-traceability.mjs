#!/usr/bin/env node
/**
 * Spec traceability gate.
 *
 * `docs/model-governance/CONTRACT.md` requires every non-null numeric limit to be
 * traceable to a `sources[].claim`, and the existing spec gate never read that
 * field — a rule that lived only in prose. This check implements it.
 *
 * Parameter limits carrying `limit_source.kind = policy_conservative` are a
 * deliberate platform policy rather than a vendor claim, so they are exempt from
 * the claim requirement (the contract says so explicitly).
 *
 * Two severities, deliberately separated:
 *
 * - **fail** — a spec declares a numeric limit but carries no `sources[]` at all.
 *   That is unambiguous: the number has no evidence behind it anywhere.
 * - **report** — a declared limit whose field carries no `source_claim`, or whose
 *   value does not literally appear in any source quote. Legacy specs predate the
 *   rule, so this is counted and printed rather than enforced; it is the backlog
 *   to work down, and the count is the number to watch.
 *
 * Usage:
 *   node scripts/ops/check-spec-traceability.mjs            # fail only on the first
 *   node scripts/ops/check-spec-traceability.mjs --strict   # enforce the second too
 */
import { readFileSync, readdirSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const SPEC_DIR = join(ROOT, 'docs/model-governance/specs')
const STRICT = process.argv.includes('--strict')

/** Fields whose numeric value is a capability limit the page may render. */
const NUMERIC_PATHS = [
  'durations_seconds.min',
  'durations_seconds.max',
  'prompt_max_chars',
  'context_window_tokens',
  'max_output_tokens',
  'max_input_bytes',
  'max_input_seconds',
  'max_output_seconds',
]

/** Objects that carry their own `source_claim`, with the limits they cover. */
const CLAIMED_OBJECTS = [
  ['reference_images', ['min', 'max', 'max_bytes_each', 'max_seconds_each']],
  ['reference_videos', ['min', 'max', 'max_bytes_each', 'max_seconds_each']],
  ['reference_audios', ['min', 'max', 'max_bytes_each', 'max_seconds_each']],
]

function readAt(object, path) {
  return path.split('.').reduce((value, key) => value?.[key], object)
}

function numericLimits(spec) {
  const limits = []
  for (const path of NUMERIC_PATHS) {
    const value = readAt(spec, path)
    if (typeof value === 'number') limits.push({ label: path, value })
  }
  for (const [field, keys] of CLAIMED_OBJECTS) {
    const holder = spec[field]
    if (!holder || typeof holder !== 'object') continue
    for (const key of keys) {
      if (typeof holder[key] === 'number') {
        limits.push({
          label: `${field}.${key}`,
          value: holder[key],
          sourceClaim: holder.source_claim,
        })
      }
    }
  }
  const parameters = Array.isArray(spec.parameters) ? spec.parameters : []
  parameters.forEach((parameter, index) => {
    if (!parameter || typeof parameter !== 'object') return
    const policyConservative = parameter.limit_source?.kind === 'policy_conservative'
    for (const key of ['minimum', 'maximum']) {
      if (typeof parameter[key] !== 'number') continue
      limits.push({
        label: `parameters[${index}].${key}`,
        value: parameter[key],
        fieldName: typeof parameter.name === 'string' ? parameter.name : '',
        policyConservative,
      })
    }
  })
  return limits
}

function main() {
  let files
  try {
    files = readdirSync(SPEC_DIR).filter((name) => name.endsWith('.json'))
  } catch (error) {
    console.error(`spec traceability: cannot read ${SPEC_DIR}: ${error.message}`)
    process.exit(1)
  }

  const unsupported = []
  const untraced = []
  let limitsSeen = 0

  for (const file of files) {
    const spec = JSON.parse(readFileSync(join(SPEC_DIR, file), 'utf8'))
    const limits = numericLimits(spec)
    limitsSeen += limits.length
    if (limits.length === 0) continue

    // `policy_conservative` limits are a deliberate platform policy, not a
    // vendor number, so they are allowed to carry no official claim. Everything
    // else must still be traceable.
    const enforceable = limits.filter((limit) => !limit.policyConservative)
    if (enforceable.length === 0) continue

    const sources = Array.isArray(spec.sources) ? spec.sources : []
    const claims = sources
      .map((source) => `${source.claim ?? ''} ${source.quote ?? ''}`)
      .join(' ')

    if (sources.length === 0) {
      unsupported.push(`${spec.model_id ?? file} declares ${enforceable.length} limits with no sources[]`)
      continue
    }

    for (const limit of enforceable) {
      const hasFieldClaim = typeof limit.sourceClaim === 'string' && limit.sourceClaim.length > 0
      const appearsInSource =
        claims.includes(String(limit.value)) ||
        (typeof limit.fieldName === 'string' && limit.fieldName !== '' && claims.includes(limit.fieldName))
      if (!hasFieldClaim && !appearsInSource) {
        untraced.push(`${spec.model_id ?? file}: ${limit.label} = ${limit.value}`)
      }
    }
  }

  console.log(
    `spec traceability: ${files.length} specs, ${limitsSeen} numeric limits, ` +
      `${unsupported.length} without any source, ${untraced.length} without a traceable claim`
  )

  if (untraced.length > 0) {
    console.log('  untraced (backlog — the count is the number to drive down):')
    for (const line of untraced.slice(0, 20)) console.log(`    ${line}`)
    if (untraced.length > 20) console.log(`    … and ${untraced.length - 20} more`)
  }

  if (unsupported.length > 0) {
    console.error('  FAIL: a declared limit with no evidence behind it:')
    for (const line of unsupported) console.error(`    ${line}`)
    process.exit(1)
  }

  if (STRICT && untraced.length > 0) {
    console.error('  FAIL (--strict): the limits above carry no traceable claim')
    process.exit(1)
  }

  console.log(STRICT ? 'all numeric limits are traceable.' : 'no spec declares a limit without sources.')
}

main()
