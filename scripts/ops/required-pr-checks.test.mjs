import assert from 'node:assert/strict'
import test from 'node:test'
import { readCheckRuns, readRequiredChecks } from './required-pr-checks.mjs'

const rule = (context, integration_id) => ({ type: 'required_status_checks', parameters: { required_status_checks: [{ context, integration_id }] } })

function api(classic, pages) {
  return async (bin, args) => {
    assert.equal(bin, 'gh')
    assert.equal(args[0], 'api')
    if (args[1] === '--paginate') {
      assert.deepEqual(args.slice(2), ['--slurp', 'repos/test/gateway/rules/branches/release%2F1?per_page=100'])
      return { stdout: JSON.stringify(pages) }
    }
    assert.deepEqual(args.slice(1), ['repos/test/gateway/branches/release%2F1/protection'])
    if (classic instanceof Error) throw classic
    return { stdout: JSON.stringify(classic) }
  }
}

test('combines policies across pages and preserves distinct app pins for the same name', async () => {
  const classic = { required_status_checks: { contexts: ['build', 'legacy'], checks: [{ context: 'build', app_id: 123 }, { context: 'lint', app_id: -1 }] } }
  const pages = [[{ type: 'merge_queue' }, rule('build', 456)], [rule('build', 123), rule('lint', null)]]
  assert.deepEqual(await readRequiredChecks('test/gateway', 'release/1', api(classic, pages)), [
    { name: 'build', appId: 123 }, { name: 'lint', appId: null },
    { name: 'legacy', appId: null }, { name: 'build', appId: 456 },
  ])
})

test('uses rulesets alone when GitHub explicitly identifies an unprotected classic branch', async () => {
  const error = Object.assign(new Error('404'), { code: 1, stderr: 'gh: Branch not protected (HTTP 404)\n' })
  assert.deepEqual(await readRequiredChecks('test/gateway', 'release/1', api(error, [[rule('ui')]])), [{ name: 'ui', appId: null }])
})

test('reads latest runs on each page with their own source, head and result', async () => {
  const sha = 'a'.repeat(40)
  const passed = { name: 'build', app: { id: 123 }, head_sha: sha, status: 'completed', conclusion: 'success' }
  const pending = { ...passed, app: { id: 456 }, status: 'in_progress', conclusion: null }
  const run = async (bin, args) => {
    assert.equal(bin, 'gh')
    assert.deepEqual(args, ['api', '--paginate', '--slurp', `repos/test/gateway/commits/${sha}/check-runs?filter=latest&per_page=100`])
    return { stdout: JSON.stringify([{ check_runs: [passed] }, { check_runs: [pending] }]) }
  }
  assert.deepEqual(await readCheckRuns('test/gateway', sha, run), [
    { name: 'build', appId: 123, status: 'COMPLETED', conclusion: 'SUCCESS' },
    { name: 'build', appId: 456, status: 'IN_PROGRESS', conclusion: null },
  ])
  passed.head_sha = 'b'.repeat(40)
  await assert.rejects(readCheckRuns('test/gateway', sha, run), /head does not match/)
})

test('an unreadable policy, empty policy or missing check name cannot signal readiness', async () => {
  const unreadable = Object.assign(new Error('Not Found'), { code: 1, stderr: 'gh: Not Found (HTTP 404)' })
  await assert.rejects(readRequiredChecks('test/gateway', 'release/1', api(unreadable, [[rule('ui')]])), /Not Found/)
  await assert.rejects(readRequiredChecks('test/gateway', 'release/1', api({}, [[]])), /No required status checks/)
  await assert.rejects(readRequiredChecks('test/gateway', 'release/1', api({}, [[rule('')]])), /Invalid required status check name/)
})
