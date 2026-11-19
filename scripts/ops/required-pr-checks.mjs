import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const exec = promisify(execFile)
const REPOSITORY = /^[\w.-]+\/[\w.-]+$/

async function readApi(run, ...args) {
  const { stdout } = await run('gh', ['api', ...args], {
    timeout: 60_000, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024,
  })
  return JSON.parse(stdout)
}

function addRequirement(requirements, name, source) {
  if (typeof name !== 'string' || !name.trim()) throw new Error('Invalid required status check name')
  const appId = source == null || source === -1 ? null : source
  if (appId !== null && (!Number.isSafeInteger(appId) || appId < 1)) {
    throw new Error('Invalid required status check app ID')
  }
  if (!requirements.some(check => check.name === name && check.appId === appId)) {
    requirements.push({ name, appId })
  }
}

export async function readRequiredChecks(repo, branch, run = exec) {
  if (!REPOSITORY.test(repo) || typeof branch !== 'string' || !branch) {
    throw new Error('Expected an owner/repo and a base branch')
  }
  const base = 'repos/' + repo
  const ref = encodeURIComponent(branch)
  const classicRequest = readApi(run, `${base}/branches/${ref}/protection`)
    .catch(error => {
      // Other 404 responses may hide permission or repository errors.
      if (error.code === 1 && error.stderr?.trim() === 'gh: Branch not protected (HTTP 404)') return {}
      throw error
    })
  const rulesRequest = readApi(run, '--paginate', '--slurp', `${base}/rules/branches/${ref}?per_page=100`)
  const [classic, pages] = await Promise.all([classicRequest, rulesRequest])
  const requirements = []
  const required = classic.required_status_checks
  if (required != null) {
    if (!Array.isArray(required.contexts)) throw new Error('Invalid classic required status checks')
    for (const check of required.checks ?? []) addRequirement(requirements, check.context, check.app_id)
    // Classic contexts mirror detailed checks without retaining their app pins.
    const detailedNames = new Set(requirements.map(check => check.name))
    for (const name of required.contexts) {
      if (!detailedNames.has(name)) addRequirement(requirements, name, null)
    }
  }
  if (!Array.isArray(pages) || !pages.length || !pages.every(Array.isArray)) {
    throw new Error('Invalid branch rules response')
  }
  for (const rule of pages.flat()) {
    if (rule.type === 'required_status_checks') {
      for (const check of rule.parameters.required_status_checks) {
        addRequirement(requirements, check.context, check.integration_id)
      }
    }
  }
  if (!requirements.length) throw new Error(`No required status checks configured for ${repo}@${branch}`)
  return requirements
}

export async function readCheckRuns(repo, sha, run = exec) {
  if (!REPOSITORY.test(repo) || !/^[0-9a-f]{40}$/i.test(sha)) throw new Error('Expected an owner/repo and a full commit SHA')
  const pages = await readApi(run, '--paginate', '--slurp', `repos/${repo}/commits/${sha}/check-runs?filter=latest&per_page=100`)
  if (!Array.isArray(pages) || !pages.length) throw new Error('Invalid check runs response')
  const checks = []
  for (const page of pages) {
    if (!Array.isArray(page.check_runs)) throw new Error('Invalid check runs page')
    for (const check of page.check_runs) {
      if (check.head_sha !== sha.toLowerCase()) throw new Error(`Check run head does not match ${sha}`)
      checks.push({
        name: check.name, appId: check.app?.id ?? null,
        status: check.status.toUpperCase(), conclusion: check.conclusion?.toUpperCase() ?? null,
      })
    }
  }
  return checks
}
