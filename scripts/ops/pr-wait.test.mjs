import assert from 'node:assert/strict'
import test from 'node:test'
import { evaluatePr, parseArgs, waitForPr } from './pr-wait.mjs'

const head = 'a'.repeat(40)
const newerHead = 'b'.repeat(40)
const success = { name: 'lint', status: 'COMPLETED', conclusion: 'SUCCESS' }
const open = { state: 'OPEN', headRefOid: head, baseRefName: 'main', statusCheckRollup: [success] }
const merged = { ...open, state: 'MERGED', mergedAt: '2026-11-19T00:00:00Z', mergeCommit: { oid: 'c'.repeat(40) } }
const args = ['--pr', '1', '--until', 'checks']

function anySource(...names) {
  return names.map(name => ({ name, appId: null }))
}

// A bounded wait advances only when the fixture returns the next snapshot.
function poll(snapshots, extraArgs = [], requiredChecks, checkRuns) {
  let time = 0
  let index = 0
  return waitForPr(parseArgs([...args, '--interval', '1', '--timeout', '2', ...extraArgs]), {
    query: () => snapshots[Math.min(index++, snapshots.length - 1)],
    now: () => time,
    sleep: ms => { time += ms },
    requiredChecks,
    checkRuns,
  })
}

test('a waiter stops when the observed PR head changes after the first poll', async () => {
  const result = await poll([{ ...open, statusCheckRollup: [] }, { ...open, headRefOid: newerHead }])
  assert.equal(result.status, 'fail')
  assert.match(result.reason, /head changed/)
})

test('a supplied head must match even on the first poll or on a merged PR', async () => {
  for (const [pr, until] of [[open, 'checks'], [merged, 'merged']]) {
    const result = await poll([pr], ['--until', until, '--head', newerHead])
    assert.equal(result.status, 'fail')
    assert.match(result.reason, /head changed/)
  }
  assert.throws(() => parseArgs([...args, '--head', 'abcd123']), /full.*SHA/)
})

test('a missing head or unknown PR state never reports success', async () => {
  for (const pr of [{ ...merged, headRefOid: null }, { ...open, state: null }]) {
    assert.equal((await poll([pr])).status, 'timeout')
  }
  assert.equal((await poll([{ ...open, headRefOid: '' }, open])).status, 'pass')
})

test('target-branch and explicit checks are combined across polls, including legacy statuses', async () => {
  const full = { ...open, statusCheckRollup: [success, { context: 'tests', state: 'SUCCESS' }, { ...success, name: 'extra' }] }
  const result = await poll([open, full], ['--repo', 'example/repo', '--required', '--require-check', 'extra'], (repo, branch) => {
    assert.deepEqual([repo, branch], ['example/repo', 'main'])
    return anySource('lint', 'tests')
  })
  assert.equal(result.status, 'pass')
  assert.equal(result.polls, 2)
  assert.equal(result.pr.headRefOid, head)
})

test('a newly required check must arrive before readiness can be reported', async () => {
  const pending = { ...open, statusCheckRollup: [] }
  const finished = { ...open, statusCheckRollup: [success, { ...success, name: 'tests' }] }
  for (const finalSnapshot of [open, finished]) {
    let reads = 0
    const result = await poll([pending, open, finalSnapshot], ['--required'], () => {
      return ++reads === 1 ? anySource('lint') : anySource('lint', 'tests')
    })
    const missing = finalSnapshot === open
    assert.equal(result.status, missing ? 'timeout' : 'pass')
    assert.deepEqual(result.checks.required, ['lint', 'tests'])
    assert.deepEqual(result.checks.missing, missing ? ['tests'] : [])
  }
})

test('a removed branch requirement no longer blocks the current head', async () => {
  let initial = true
  const result = await poll([open], ['--required'], () => {
    if (!initial) return anySource('lint')
    initial = false
    return anySource('lint', 'retired')
  })
  assert.equal(result.status, 'pass')
  assert.deepEqual(result.checks.required, ['lint'])
})

test('an unavailable policy refresh cannot return a cached pass', async () => {
  let reads = 0
  await assert.rejects(poll([{ ...open, statusCheckRollup: [] }, open], ['--required'], () => {
    if (++reads === 1) return anySource('lint')
    throw new Error('policy refresh unavailable')
  }), /policy refresh unavailable/)
})

test('a check with the right name must come from each required app on the observed head', async () => {
  const sources = [{ name: 'lint', appId: 4 }, { name: 'lint', appId: 8 }]
  const first = { ...success, appId: 4 }
  const second = { ...success, appId: 8 }
  for (const [runs, status] of [
    [[{ ...success, appId: 100 }], 'timeout'],
    [[first], 'timeout'],
    [[first, { ...second, status: 'in_progress', conclusion: null }], 'timeout'],
    [[first, { ...second, conclusion: 'failure' }], 'fail'],
    [[first, second, { ...success, appId: 100, conclusion: 'failure' }], 'pass'],
  ]) {
    const result = await poll([open], ['--required', '--require-check', 'lint', '--repo', 'example/repo'], () => sources, (repo, sha) => {
      assert.deepEqual([repo, sha], ['example/repo', head])
      return runs
    })
    assert.equal(result.status, status)
    assert.deepEqual(result.checks.requiredSources, sources)
    if (status !== 'pass') assert.match(result.reason, /app 8/)
  }
})

test('a successful app check cannot hide a failing or pending PR status with the same name', async () => {
  const policy = () => [{ name: 'lint', appId: 4 }]
  const runs = () => [{ ...success, appId: 4 }]
  for (const [entry, expected] of [
    [{ context: 'lint', state: 'FAILURE' }, 'fail'],
    [{ context: 'lint', state: 'PENDING' }, 'timeout'],
    [{ ...success, conclusion: 'FAILURE' }, 'fail'],
    [{ context: 'lint', state: 'SUCCESS' }, 'pass'],
  ]) {
    const pr = { ...open, statusCheckRollup: [success, entry] }
    const result = await poll([pr], ['--required'], policy, runs)
    assert.equal(result.status, expected)
  }
})

test('a partial rollup, unknown state or missing conclusion cannot complete the wait', async () => {
  const explicit = ['--require-check', 'lint', '--require-check', 'tests']
  for (const check of [null, { context: 'tests', state: 'PENDING' }, { ...success, name: 'tests', conclusion: 'FUTURE' },
    { name: 'tests', status: 'COMPLETED' }]) {
    const pr = { ...open, statusCheckRollup: check ? [success, check] : [success] }
    const result = await poll([pr], explicit)
    assert.equal(result.status, 'timeout')
    assert.match(result.reason, /tests/)
  }
})

test('skipped or neutral checks only count as settled when optional', async () => {
  for (const conclusion of ['SKIPPED', 'NEUTRAL']) {
    const pr = { ...open, statusCheckRollup: [success, { ...success, name: 'docs', conclusion }] }
    assert.equal((await poll([pr], ['--require-check', 'lint'])).status, 'pass')
    assert.equal((await poll([pr], ['--require-check', 'docs'])).status, 'fail')
  }
})

test('a retargeted PR or unreadable target policy cannot reuse earlier check coverage', async () => {
  const result = await poll([open, { ...open, baseRefName: 'release' }], ['--required'], () => anySource('tests'))
  assert.equal(result.status, 'fail')
  assert.match(result.reason, /base changed/)
  await assert.rejects(poll([open], ['--required'], () => []), /no configured required checks/)
  await assert.rejects(poll([open], ['--required'], () => { throw new Error('unavailable') }), /unavailable/)
})

test('a merge receipt proves identity independently of PR test results', async () => {
  const noChecks = { ...merged, statusCheckRollup: [] }
  assert.equal((await poll([noChecks], ['--require-check', 'tests'])).status, 'timeout')
  const result = await poll([noChecks], ['--until', 'merged', '--head', head])
  assert.equal(result.reason, 'merged')
  assert.equal(result.pr.mergeCommit, merged.mergeCommit.oid)
  assert.equal(evaluatePr({ ...merged, mergedAt: null }, 'merged').status, 'fail')
  assert.equal(evaluatePr({ ...merged, mergeCommit: null }, 'merged').status, 'fail')
  assert.equal(evaluatePr({ ...open, state: 'CLOSED' }, 'merged').status, 'fail')
  assert.throws(() => parseArgs(['--pr', '1', '--until', 'merged', '--required']), /only valid with --until checks/)
})
