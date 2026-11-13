const { test } = require('node:test');
const assert = require('node:assert/strict');
const { run } = require('../src/commands/social');

const commands = [
  ['accounts'], ['connect', '--platform', 'tiktok', '--no-open'],
  ['disconnect', '1'], ['post', '--accounts', '1', '--text', 'hello'],
  ['post-status', '9', '--no-refresh'],
];

test('social commands reject missing or unknown provider before any request', async () => {
  for (const command of commands) {
    for (const extra of [[], ['--provider', 'unknown']]) {
      const calls = [];
      await assert.rejects(run({ args: [...command, ...extra], flags: {}, api: async (...args) => { calls.push(args); return {}; } }), /provider/);
      assert.deepEqual(calls, []);
    }
  }
});

test('social commands send the explicit provider in the correct request location', async () => {
  for (const provider of ['zernio', 'tiktok_direct']) {
    for (const command of commands) {
      const calls = [];
      await run({ args: [...command, '--provider', provider], flags: {}, api: async (...args) => { calls.push(args); return { success: true, data: {} }; } });
      const call = calls.find(([, url]) => !url.endsWith('/seats'));
      assert.ok(call);
      if (call[0] === 'POST') assert.equal(call[2].provider, provider);
      else assert.equal(new URL(call[1], 'http://fixture').searchParams.get('provider'), provider);
      if (command[0] === 'post-status') assert.equal(new URL(call[1], 'http://fixture').searchParams.get('refresh'), '0');
    }
  }
});

test('official connect rejects other platforms before seat preflight', async () => {
  const calls = [];
  await assert.rejects(run({ args: ['connect', '--provider=tiktok_direct', '--platform=instagram'], flags: {}, api: async (...args) => { calls.push(args); return {}; } }), /TikTok/);
  assert.deepEqual(calls, []);
});
