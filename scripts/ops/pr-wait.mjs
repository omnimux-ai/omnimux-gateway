#!/usr/bin/env node
/**
 * pr-wait — bounded, background-friendly waiter for PR checks and merges.
 *
 * Why: an agent that polls with `sleep 60; gh pr view` burns whole model turns
 * on wall-clock waiting. Run this once through the harness background-job
 * facility; it polls inside one process and exits when the condition is met,
 * failed, or timed out. The completion notification is the resume trigger.
 *
 * Usage:
 *   node scripts/ops/pr-wait.mjs --pr <number> --until checks|merged [--json]
 *     [--head full-sha] [--repo owner/name] [--interval seconds] [--timeout seconds]
 *   For checks: [--required] [--require-check name ...]
 *   --required uses the target branch's configured checks. Each named check
 *   must report SUCCESS. A merge receipt is a separate condition.
 *
 * Exit codes: 0 = condition reached, 1 = failed/closed, 2 = timeout or error.
 */
import { spawnSync } from 'node:child_process'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { readCheckRuns, readRequiredChecks } from './required-pr-checks.mjs'

const DEFAULT_REPO = process.env.OMNIMUX_PR_REPO || 'omnimux-ai/omnimux-gateway'
const DEFAULT_INTERVAL_SECONDS = 20
const DEFAULT_TIMEOUT_SECONDS = 1800

const FAILURE_STATES = new Set([
  'FAILURE',
  'FAILED',
  'ERROR',
  'CANCELLED',
  'TIMED_OUT',
  'ACTION_REQUIRED',
  'STARTUP_FAILURE',
  'STALE',
])
const OPTIONAL_CONCLUSIONS = new Set(['NEUTRAL', 'SKIPPED'])
const FULL_SHA = /^[0-9a-f]{40}$/i

function positiveInteger(value, name) {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) throw new Error(`${name} must be a positive integer`)
  return parsed
}

export function parseArgs(argv) {
  const options = {
    pr: '',
    until: '',
    head: '',
    required: false,
    requiredChecks: [],
    repo: DEFAULT_REPO,
    intervalMs: DEFAULT_INTERVAL_SECONDS * 1000,
    timeoutMs: DEFAULT_TIMEOUT_SECONDS * 1000,
    json: false,
  }
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i]
    const next = () => {
      if (argv[i + 1] === undefined || argv[i + 1].startsWith('--')) throw new Error(`missing value for ${arg}`)
      i += 1
      return argv[i]
    }
    if (arg === '--pr') options.pr = String(positiveInteger(next(), '--pr'))
    else if (arg === '--until') {
      const until = next()
      if (until !== 'checks' && until !== 'merged') throw new Error('--until must be checks or merged')
      options.until = until
    } else if (arg === '--head') {
      options.head = next().toLowerCase()
      if (!FULL_SHA.test(options.head)) throw new Error('--head must be a full 40-character commit SHA')
    } else if (arg === '--required') options.required = true
    else if (arg === '--require-check') {
      const name = next()
      if (!name.trim()) throw new Error('--require-check must name a check')
      options.requiredChecks.push(name)
    } else if (arg === '--repo') options.repo = next()
    else if (arg === '--interval') options.intervalMs = positiveInteger(next(), '--interval') * 1000
    else if (arg === '--timeout') options.timeoutMs = positiveInteger(next(), '--timeout') * 1000
    else if (arg === '--json') options.json = true
    else throw new Error(`unknown argument: ${arg}`)
  }
  if (!options.pr) throw new Error('--pr is required')
  if (!options.until) throw new Error('--until is required')
  if (options.until === 'merged' && (options.required || options.requiredChecks.length)) {
    throw new Error('--required and --require-check are only valid with --until checks')
  }
  return options
}

/** Normalize both check-run and status-context entries into one verdict. */
export function summarizeChecks(rollup, requiredChecks = [], requiredSources = [], checkRuns = []) {
  const entries = Array.isArray(rollup) ? rollup : []
  const sourceNames = new Set(requiredSources.map(source => source.name))
  // Matching the specified app also satisfies an explicit name-only request.
  const required = new Set(requiredChecks.filter(name => !sourceNames.has(name)))
  const missing = new Set(required)
  const missingSources = []
  const observations = []
  for (const entry of entries) {
    const name = entry?.name || entry?.context
    // A separate commit status with the same name can still be pending or
    // failed even when the required app's check run has succeeded.
    missing.delete(name)
    observations.push({ entry, label: name || 'unnamed', strict: required.has(name) })
  }
  for (const { name, appId } of requiredSources) {
    const label = `${name} (app ${appId})`
    const matches = checkRuns.filter(run => run.name === name && run.appId === appId)
    if (matches.length === 0) missingSources.push(label)
    for (const entry of matches) observations.push({ entry, label, strict: true })
  }
  const failures = []
  const pending = []
  for (const { entry, label, strict } of observations) {
    // CheckRun has a status and a terminal conclusion. StatusContext uses state.
    const status = entry?.status?.toUpperCase()
    const state = String(status && status !== 'COMPLETED'
      ? status
      : entry?.conclusion || entry?.state || '').toUpperCase()
    if (state === 'SUCCESS') continue
    if (OPTIONAL_CONCLUSIONS.has(state) && !strict) continue
    const result = `${label}=${state || 'UNKNOWN'}`
    if (FAILURE_STATES.has(state) || OPTIONAL_CONCLUSIONS.has(state)) failures.push(result)
    else pending.push(result)
  }
  return {
    total: entries.length, hasChecks: entries.length > 0, failures, pending,
    required: [...required, ...sourceNames], requiredSources, missing: [...missing, ...missingSources],
  }
}

/**
 * Evaluate a `gh pr view` snapshot.
 * @param {object|null} pr
 * @param {'checks'|'merged'} until
 * @param {{head?: string, base?: string, requiredChecks?: string[], requiredSources?: object[], checkRuns?: object[]}} expected
 */
export function evaluatePr(pr, until, expected = {}) {
  const checks = summarizeChecks(pr?.statusCheckRollup, expected.requiredChecks, expected.requiredSources, expected.checkRuns)
  const state = String(pr?.state || '').toUpperCase()

  if (state === 'CLOSED') return { status: 'fail', reason: 'PR closed without merge', checks }
  if (!FULL_SHA.test(pr?.headRefOid)) return { status: 'wait', reason: 'PR head unavailable', checks }
  if (expected.head && expected.head.toLowerCase() !== pr.headRefOid.toLowerCase()) {
    return { status: 'fail', reason: `PR head changed from ${expected.head} to ${pr.headRefOid}`, checks }
  }
  if (expected.base && expected.base !== pr.baseRefName) {
    return { status: 'fail', reason: `PR base changed from ${expected.base} to ${pr.baseRefName || 'unknown'}`, checks }
  }
  if (state === 'MERGED') {
    if (!pr.mergedAt || !FULL_SHA.test(pr.mergeCommit?.oid)) {
      return { status: 'fail', reason: 'MERGED without merge identity', checks }
    }
    if (until === 'merged') return { status: 'pass', reason: 'merged', checks }
  } else if (state !== 'OPEN') return { status: 'wait', reason: 'PR state unavailable', checks }
  if (checks.failures.length > 0) {
    return { status: 'fail', reason: `checks failed: ${checks.failures.join(', ')}`, checks }
  }
  if (until === 'checks') {
    if (checks.missing.length > 0) return { status: 'wait', reason: `checks missing: ${checks.missing.join(', ')}`, checks }
    if (!checks.hasChecks) return { status: 'wait', reason: 'no checks reported yet', checks }
    if (checks.pending.length > 0) return { status: 'wait', reason: `checks pending: ${checks.pending.join(', ')}`, checks }
    return { status: 'pass', reason: 'checks passed', checks }
  }
  return { status: 'wait', reason: `state=${state || 'UNKNOWN'}`, checks }
}

function snapshot(pr) {
  return {
    number: pr?.number ?? null,
    url: pr?.url ?? null,
    state: pr?.state ?? null,
    headRefOid: pr?.headRefOid ?? null,
    baseRefName: pr?.baseRefName ?? null,
    mergedAt: pr?.mergedAt ?? null,
    mergeCommit: pr?.mergeCommit?.oid ?? null,
  }
}

export function queryPr(prNumber, repo) {
  const result = spawnSync('gh', [
    'pr', 'view', String(prNumber), '--repo', repo, '--json',
    'number,url,state,headRefOid,baseRefName,statusCheckRollup,mergedAt,mergeCommit',
  ], { encoding: 'utf8', timeout: 60_000 })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error((result.stderr || result.stdout || 'gh pr view failed').trim())
  return JSON.parse(result.stdout)
}

/**
 * Poll one PR until the requested condition settles.
 * @param {{pr: string, until: 'checks'|'merged', repo: string, intervalMs: number, timeoutMs: number, head?: string, required?: boolean, requiredChecks?: string[]}} options
 * @param {{query?: Function, requiredChecks?: Function, checkRuns?: Function, sleep?: Function, now?: Function, log?: Function}} deps
 */
export async function waitForPr(options, deps = {}) {
  const query = deps.query || queryPr
  const sleep = deps.sleep || (ms => new Promise(resolvePromise => setTimeout(resolvePromise, ms)))
  const now = deps.now || (() => Date.now())
  const log = deps.log || (() => {})
  const startedAt = now()
  let polls = 0
  let lastReason = ''
  const expected = { head: options.head?.toLowerCase(), base: '', requiredChecks: options.requiredChecks || [] }

  for (;;) {
    polls += 1
    const pr = await query(options.pr, options.repo)
    const observedHead = FULL_SHA.test(pr?.headRefOid) ? pr.headRefOid.toLowerCase() : ''
    expected.head ||= observedHead
    const matchesHead = observedHead && observedHead === expected.head
    const matchesBase = !expected.base || pr?.baseRefName === expected.base
    // A branch can add or retire a check without changing the PR head or base.
    if (options.required && matchesHead && matchesBase && pr.state !== 'CLOSED') {
      if (!pr.baseRefName) throw new Error('cannot verify required checks without the PR base branch')
      expected.base = pr.baseRefName
      const configured = await (deps.requiredChecks || readRequiredChecks)(options.repo, expected.base)
      if (!Array.isArray(configured) || configured.length === 0) {
        throw new Error(`no configured required checks found for ${options.repo}:${expected.base}`)
      }
      expected.requiredChecks = [...(options.requiredChecks || []), ...configured.filter(rule => rule.appId === null).map(rule => rule.name)]
      expected.requiredSources = configured.filter(rule => rule.appId !== null)
      expected.checkRuns = expected.requiredSources.length ? await (deps.checkRuns || readCheckRuns)(options.repo, observedHead) : []
    }
    const verdict = evaluatePr(pr, options.until, expected)
    if (verdict.reason !== lastReason) {
      log(`[pr-wait] #${options.pr} ${verdict.status}: ${verdict.reason} (poll ${polls})`)
      lastReason = verdict.reason
    }
    if (verdict.status === 'pass') {
      return { ok: true, status: 'pass', reason: verdict.reason, checks: verdict.checks, polls, elapsedMs: now() - startedAt, pr: snapshot(pr) }
    }
    if (verdict.status === 'fail') {
      return { ok: false, status: 'fail', reason: verdict.reason, checks: verdict.checks, polls, elapsedMs: now() - startedAt, pr: snapshot(pr) }
    }
    if (now() - startedAt >= options.timeoutMs) {
      return { ok: false, status: 'timeout', reason: `timed out after ${Math.round(options.timeoutMs / 1000)}s: ${verdict.reason}`, checks: verdict.checks, polls, elapsedMs: now() - startedAt, pr: snapshot(pr) }
    }
    const remaining = options.timeoutMs - (now() - startedAt)
    await sleep(Math.min(options.intervalMs, remaining))
  }
}

export async function main(argv = process.argv.slice(2), deps = {}) {
  let options
  try {
    options = parseArgs(argv)
  } catch (error) {
    process.stderr.write(`[pr-wait] ${error.message}\n`)
    return 2
  }
  let result
  try {
    result = await waitForPr(options, {
      ...deps,
      log: deps.log || (line => (options.json ? process.stderr : process.stdout).write(`${line}\n`)),
    })
  } catch (error) {
    process.stderr.write(`[pr-wait] ERROR ${error.message}\n`)
    return 2
  }
  if (options.json) process.stdout.write(`${JSON.stringify(result, null, 2)}\n`)
  else process.stdout.write(`[pr-wait] ${result.status.toUpperCase()} ${result.reason} (${result.polls} polls, ${Math.round(result.elapsedMs / 1000)}s)\n`)
  return result.ok ? 0 : result.status === 'timeout' ? 2 : 1
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  process.exitCode = await main()
}
