const { test } = require('node:test');
const assert = require('node:assert/strict');
const { run } = require('../src/commands/social');

const providerRequiredCommands = [
  ['accounts'],
  ['connect', '--platform', 'tiktok', '--no-open'],
  ['disconnect', '1'],
  ['post', '--accounts', '1', '--text', 'hello'],
  ['post-status', '9', '--no-refresh'],
];

const allProviderCommands = [
  ...providerRequiredCommands,
  ['creator-info', '12'],
];

test('social commands reject missing or unknown provider before any request', async () => {
  for (const command of providerRequiredCommands) {
    for (const extra of [[], ['--provider', 'unknown']]) {
      const calls = [];
      await assert.rejects(
        run({
          args: [...command, ...extra],
          flags: {},
          api: async (...args) => {
            calls.push(args);
            return {};
          },
        }),
        /provider/
      );
      assert.deepEqual(calls, []);
    }
  }
});

test('retired --provider zernio fails locally with migration guidance before any request', async () => {
  for (const command of allProviderCommands) {
    for (const val of ['zernio', 'ZERNIO']) {
      const calls = [];
      await assert.rejects(
        run({
          args: [...command, '--provider', val],
          flags: {},
          api: async (...args) => {
            calls.push(args);
            return {};
          },
        }),
        /zernio.*retired.*tiktok_direct/i
      );
      assert.deepEqual(calls, []);
    }
  }
});

test('social commands send official tiktok_direct provider in the correct request location', async () => {
  for (const command of allProviderCommands) {
    const calls = [];
    await run({
      args: [...command, '--provider', 'tiktok_direct'],
      flags: {},
      api: async (...args) => {
        calls.push(args);
        return { success: true, data: {} };
      },
    });
    const call = calls.find(([, url]) => !url.endsWith('/seats'));
    assert.ok(call);
    if (call[0] === 'POST') assert.equal(call[2].provider, 'tiktok_direct');
    else assert.equal(new URL(call[1], 'http://fixture').searchParams.get('provider'), 'tiktok_direct');
    if (command[0] === 'post-status') {
      assert.equal(new URL(call[1], 'http://fixture').searchParams.get('refresh'), '0');
    }
    if (command[0] === 'creator-info') {
      assert.equal(call[0], 'GET');
      assert.equal(call[1], '/api/social/v1/accounts/12/creator-info?provider=tiktok_direct');
    }
  }
});

test('social creator-info defaults to tiktok_direct and validates accountId/provider locally', async () => {
  const calls = [];
  await run({
    args: ['creator-info', '42'],
    flags: {},
    api: async (...args) => {
      calls.push(args);
      return { success: true, data: { creator_username: 'demo' } };
    },
  });
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], 'GET');
  assert.equal(calls[0][1], '/api/social/v1/accounts/42/creator-info?provider=tiktok_direct');

  const badCalls = [];
  await assert.rejects(
    run({
      args: ['creator-info'],
      flags: {},
      api: async (...args) => {
        badCalls.push(args);
        return {};
      },
    }),
    /creator-info <accountId>/
  );
  await assert.rejects(
    run({
      args: ['creator-info', '42', '--provider', 'unknown'],
      flags: {},
      api: async (...args) => {
        badCalls.push(args);
        return {};
      },
    }),
    /tiktok_direct/
  );
  assert.deepEqual(badCalls, []);
});

test('social post forwards tiktok_options including zero photo_cover_index, false is_aigc, and compliance flags', async () => {
  const calls = [];
  await run({
    args: [
      'post',
      '--provider',
      'tiktok_direct',
      '--accounts',
      '7',
      '--media',
      'https://cdn.example.com/1.jpg,https://cdn.example.com/2.webp',
      '--title',
      'Spring Lookbook',
      '--description',
      'Two slide carousel',
      '--photo-cover-index',
      '0',
      '--is-aigc=false',
      '--privacy-level',
      'SELF_ONLY',
      '--no-allow-comment',
      '--allow-duet=false',
      '--allow-stitch',
      'true',
      '--commercial-content',
      '--your-brand',
      '--no-branded-content',
      '--music-usage-confirmed',
    ],
    flags: {},
    api: async (...args) => {
      calls.push(args);
      return { success: true, data: { id: 1 } };
    },
  });
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], 'POST');
  assert.equal(calls[0][1], '/api/social/v1/posts');
  assert.deepEqual(calls[0][2].tiktok_options, {
    privacy_level: 'SELF_ONLY',
    allow_comment: false,
    allow_duet: false,
    allow_stitch: true,
    commercial_content: true,
    your_brand: true,
    branded_content: false,
    music_usage_confirmed: true,
    title: 'Spring Lookbook',
    description: 'Two slide carousel',
    photo_cover_index: 0,
    is_aigc: false,
  });
});

test('social post merges --tiktok-options JSON with explicit CLI flags', async () => {
  const calls = [];
  await run({
    args: [
      'post',
      '--provider',
      'tiktok_direct',
      '--accounts',
      '7',
      '--media',
      'https://cdn.example.com/1.jpg',
      '--tiktok-options',
      '{"title":"From JSON","is_aigc":true,"photo_cover_index":0}',
      '--no-is-aigc',
    ],
    flags: {},
    api: async (...args) => {
      calls.push(args);
      return { success: true, data: { id: 2 } };
    },
  });
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0][2].tiktok_options, {
    title: 'From JSON',
    is_aigc: false,
    photo_cover_index: 0,
  });
});

test('official connect rejects other platforms before seat preflight', async () => {
  const calls = [];
  await assert.rejects(
    run({
      args: ['connect', '--provider=tiktok_direct', '--platform=instagram'],
      flags: {},
      api: async (...args) => {
        calls.push(args);
        return {};
      },
    }),
    /TikTok/
  );
  assert.deepEqual(calls, []);
});
