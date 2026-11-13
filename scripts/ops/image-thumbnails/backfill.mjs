#!/usr/bin/env node
import { createRequire } from 'node:module'
import { readFile, writeFile, rename } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export function parseArguments(args) {
  const value = (name, fallback) => args.find((arg) => arg.startsWith(`--${name}=`))?.slice(name.length + 3) ?? fallback
  const base = value('base', '')
  if (!base) throw new Error('--base is required; there is no production default')
  const url = new URL(base)
  if (url.username || url.password || url.search || url.hash || !['http:', 'https:'].includes(url.protocol)) throw new Error('Invalid base URL')
  if (url.protocol !== 'https:' && !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)) throw new Error('Remote base must use HTTPS')
  const limit = Number(value('limit', '100'))
  const maxBatches = Number(value('max-batches', '100'))
  if (!Number.isInteger(limit) || limit < 1 || limit > 100 || !Number.isInteger(maxBatches) || maxBatches < 1) throw new Error('Invalid batch bounds')
  return { base: url.toString().replace(/\/$/, ''), apply: args.includes('--apply'), limit, maxBatches, checkpoint: path.resolve(value('checkpoint', 'image-thumbnail-checkpoint.json')) }
}

export async function fetchWithRateLimitRetry(send, wait) {
  for (let attempt = 0; attempt < 4; attempt++) {
    const response = await send()
    if (response.status !== 429 || attempt === 3) return response
    const raw = response.headers.get('retry-after')
    let seconds = Number(raw)
    if (raw && !Number.isFinite(seconds)) seconds = Math.max(0, (Date.parse(raw) - Date.now()) / 1000)
    if (!raw || !Number.isFinite(seconds)) seconds = 10
    // Wait in short chunks, preserving the server's full rate-limit window.
    let remaining = Math.max(1000, Math.ceil(seconds * 1000))
    if (remaining > 20 * 60_000) throw new Error('Rate-limit window exceeds 20 minutes; resume from checkpoint later')
    while (remaining > 0) { const step = Math.min(remaining, 60_000); await wait(step); remaining -= step }
  }
}

export async function runBackfill(options, request, checkpoint, save, wait) {
  if (checkpoint.base !== options.base || checkpoint.apply !== options.apply) throw new Error('Checkpoint target/mode mismatch')
  for (let batch = 0; batch < options.maxBatches && checkpoint.scope !== 'done'; batch++) {
    const query = { scope: checkpoint.scope, before: checkpoint.before, limit: options.limit, dry_run: !options.apply }
    let result = await request(query)
    if (options.apply) {
      // Inspect only while queued jobs run; never re-enqueue failed work from polling.
      for (let poll = 0; poll < 18 && result.items.some((item) => !['ready', 'failed'].includes(item.status)); poll++) {
        await wait(10_000)
        result = await request({ ...query, dry_run: true })
        // Queue saturation is recoverable: only explicit apply calls admit more identifiers.
        if (result.items.some((item) => item.status === 'missing')) result = await request(query)
      }
      if (result.items.some((item) => !['ready', 'failed'].includes(item.status))) {
        await save(checkpoint)
        throw new Error('Batch still pending; checkpoint retained at the same page for resume')
      }
    }
    checkpoint.failures.push(...result.items.filter((item) => item.status === 'failed').map(({ id, task_id, artifact_key, reason }) => ({ scope: checkpoint.scope, id, task_id, artifact_key, reason })))
    checkpoint.scanned += result.scanned
    checkpoint.ready += result.items.filter((item) => item.status === 'ready').length
    checkpoint.missing += result.items.filter((item) => item.status === 'missing').length
    if (result.next_before) checkpoint.before = result.next_before
    else { checkpoint.scope = checkpoint.scope === 'presets' ? 'tasks' : 'done'; checkpoint.before = 0 }
    await save(checkpoint)
  }
  return checkpoint
}

async function main() {
  const options = parseArguments(process.argv.slice(2))
  const require = createRequire(import.meta.url)
  const { BASE_URL, ACCESS_TOKEN, USER_ID, assertConfig } = require('../../../.agents/skills/newapi/scripts/env.js')
  assertConfig({ requireAuth: true })
  if (BASE_URL.replace(/\/$/, '') !== options.base) throw new Error('Explicit base must match the selected newapi credential profile')
  let checkpoint
  try { checkpoint = JSON.parse(await readFile(options.checkpoint, 'utf8')) }
  catch (error) { if (error.code !== 'ENOENT') throw error; checkpoint = { base: options.base, apply: options.apply, scope: 'presets', before: 0, scanned: 0, ready: 0, missing: 0, failures: [] } }
  const save = async (state) => {
    await writeFile(`${options.checkpoint}.tmp`, JSON.stringify(state, null, 2), { mode: 0o600 })
    await rename(`${options.checkpoint}.tmp`, options.checkpoint)
    console.log(JSON.stringify({ scope: state.scope, before: state.before, scanned: state.scanned, ready: state.ready, missing: state.missing, failed: state.failures.length }))
  }
  const request = async (query) => {
    const response = await fetchWithRateLimitRetry(() => fetch(`${options.base}/api/system-task/image-thumbnail-backfill`, {
      method: 'POST', redirect: 'error', signal: AbortSignal.timeout(30_000),
      headers: { authorization: `Bearer ${ACCESS_TOKEN}`, 'New-Api-User': String(USER_ID), 'content-type': 'application/json' },
      body: JSON.stringify(query),
    }), (ms) => new Promise((resolve) => setTimeout(resolve, ms)))
    if (!response.ok) throw new Error(`Backfill HTTP ${response.status}`)
    const payload = await response.json()
    if (!payload.success || !Array.isArray(payload.data?.items)) throw new Error('Backfill rejected or invalid response')
    return payload.data
  }
  const result = await runBackfill(options, request, checkpoint, save, (ms) => new Promise((resolve) => setTimeout(resolve, ms)))
  if (result.scope !== 'done') throw new Error('Batch limit reached; resume using the same checkpoint')
  if (result.failures.length) process.exitCode = 2
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main().catch((error) => { console.error(error.message); process.exitCode = 1 })
