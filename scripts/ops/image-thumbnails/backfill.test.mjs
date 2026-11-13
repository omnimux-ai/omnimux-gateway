import test from 'node:test'
import assert from 'node:assert/strict'
import { parseArguments, runBackfill, fetchWithRateLimitRetry } from './backfill.mjs'

test('backfill requires an explicit destination and defaults to inspection', () => {
  assert.throws(() => parseArguments([]))
  assert.equal(parseArguments(['--base=http://localhost:3819']).apply, false)
  assert.throws(() => parseArguments(['--base=http://remote.example']))
  assert.throws(() => parseArguments(['--base=https://host.example', '--limit=101']))
})

test('apply checkpoint advances only after thumbnails become ready and preserves failures', async () => {
 const options = parseArguments(['--base=http://localhost:3819', '--apply'])
 const checkpoint = { base: options.base, apply: true, scope: 'presets', before: 0, scanned: 0, ready: 0, missing: 0, failures: [] }
 const queries=[]
 const responses = [
  {scanned:1,next_before:0,items:[{id:1,status:'queued'}]},
  {scanned:1,next_before:0,items:[{id:1,status:'ready'}]},
  {scanned:1,next_before:0,items:[{id:2,status:'failed',reason:'source expired'}]},
 ]
 await runBackfill(options,async(query)=>{queries.push(query);return responses.shift()},checkpoint,async()=>{},async()=>{})
 assert.equal(checkpoint.scope,'done')
 assert.equal(checkpoint.ready,1)
 assert.equal(checkpoint.failures.length,1)
 assert.deepEqual(queries.map(q=>q.dry_run),[false,true,false])
})

test('unknown readiness retains current page for a resumable failure', async () => {
 const options = parseArguments(['--base=http://localhost:3819','--apply'])
 const checkpoint = {base: options.base,apply:true,scope:'tasks',before:50,scanned:0,ready:0,missing:0,failures:[]}
 await assert.rejects(runBackfill(options,async()=>({scanned:1,next_before:25,items:[{id:40,status:'unknown'}]}),checkpoint,async()=>{},async()=>{}),/pending/)
 assert.equal(checkpoint.before,50)
 assert.equal(checkpoint.scanned,0)
})

 test('rate limiting waits for Retry-After before retrying the same operation', async () => {
  let calls=0; const waits=[]
  const response=await fetchWithRateLimitRetry(async()=>{calls++;return new Response(null,{status:calls===1?429:200,headers:{'retry-after':'120'}})},async(ms)=>{waits.push(ms)})
  assert.equal(response.status,200)
  assert.equal(calls,2)
  assert.deepEqual(waits,[60000,60000])
 })
