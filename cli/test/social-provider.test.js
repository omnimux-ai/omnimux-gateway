const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { run, uploadLocalFile } = require('../src/commands/social');
const { uploadLocalFile: uploadHostedLocalFile } = require('../src/commands/hosted/shared');
const { putBinary } = require('../src/lib/upload');

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

test('social uploadLocalFile forwards presigned signed headers (If-None-Match) and strips Host over real HTTP', async () => {
  const received = [];
  const server = http.createServer((req, res) => {
    const chunks = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => {
      received.push({
        method: req.method,
        url: req.url,
        headers: { ...req.headers },
        body: Buffer.concat(chunks),
      });
      if (req.headers['if-none-match'] !== '*') {
        res.writeHead(403, { 'content-type': 'application/xml' });
        res.end('<Error><Code>SignatureDoesNotMatch</Code></Error>');
        return;
      }
      res.writeHead(200);
      res.end();
    });
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'omnimux-cli-upload-'));
  const filePath = path.join(tmpDir, 'clip.mp4');
  const payload = Buffer.from('fixture-mp4-bytes');
  fs.writeFileSync(filePath, payload);

  try {
    const res = await uploadLocalFile(
      {
        api: async (method, urlPath, body) => {
          assert.equal(method, 'POST');
          assert.equal(urlPath, '/api/social/v1/media/presign');
          assert.equal(body.filename, 'clip.mp4');
          assert.equal(body.content_type, 'video/mp4');
          assert.equal(body.size_bytes, payload.length);
          return {
            success: true,
            data: {
              upload_url: `http://127.0.0.1:${port}/social/1/clip.mp4`,
              upload_method: 'PUT',
              public_url: 'https://files.example.test/social/1/clip.mp4',
              headers: {
                Host: 'bucket.r2.example.invalid',
                'Content-Type': 'video/mp4',
                'If-None-Match': '*',
                'X-Amz-Meta-Source': 'omnimux',
              },
            },
          };
        },
      },
      filePath
    );

    assert.equal(res.public_url, 'https://files.example.test/social/1/clip.mp4');
    assert.equal(received.length, 1);
    assert.equal(received[0].method, 'PUT');
    assert.equal(received[0].headers['if-none-match'], '*');
    assert.equal(received[0].headers['content-type'], 'video/mp4');
    assert.equal(received[0].headers['x-amz-meta-source'], 'omnimux');
    assert.equal(received[0].headers.host, `127.0.0.1:${port}`);
    assert.deepEqual(received[0].body, payload);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
    await new Promise((resolve) => server.close(resolve));
  }
});

test('putBinary and hosted uploadLocalFile preserve fallback Content-Type and forward optional signed headers', async () => {
  const received = [];
  const server = http.createServer((req, res) => {
    req.resume();
    req.on('end', () => {
      received.push({ method: req.method, headers: { ...req.headers } });
      res.writeHead(200);
      res.end();
    });
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'omnimux-cli-hosted-'));
  const filePath = path.join(tmpDir, 'photo.jpg');
  fs.writeFileSync(filePath, Buffer.from('jpg-bytes'));

  try {
    // 1. Default PUT without opts.headers falls back to contentType argument
    await putBinary(`http://127.0.0.1:${port}/default`, Buffer.from('a'), 'image/png');
    assert.equal(received[0].method, 'PUT');
    assert.equal(received[0].headers['content-type'], 'image/png');

    // 2. Signed content-type wins over fallback; empty/non-string values and Host are ignored
    await putBinary(`http://127.0.0.1:${port}/signed`, Buffer.from('b'), 'application/octet-stream', {
      method: 'put',
      headers: {
        HOST: 'wrong.invalid',
        'Content-Type': 'image/jpeg',
        'If-None-Match': '*',
        'X-Empty': '',
        'X-Num': 123,
      },
    });
    assert.equal(received[1].method, 'PUT');
    assert.equal(received[1].headers['content-type'], 'image/jpeg');
    assert.equal(received[1].headers['if-none-match'], '*');
    assert.equal(received[1].headers['x-empty'], undefined);
    assert.equal(received[1].headers['x-num'], undefined);
    assert.equal(received[1].headers.host, `127.0.0.1:${port}`);

    // 3. Hosted uploadLocalFile forwards optional presign headers when provided
    const hostedRes = await uploadHostedLocalFile(
      {
        api: async () => ({
          success: true,
          data: {
            upload_url: `http://127.0.0.1:${port}/hosted/photo.jpg`,
            resource_url: 'https://cdn.example.test/photo.jpg',
            headers: { Host: 'wrong.invalid', 'If-None-Match': '*', 'Content-Type': 'image/jpeg' },
          },
        }),
      },
      filePath
    );
    assert.equal(hostedRes.resource_url, 'https://cdn.example.test/photo.jpg');
    assert.equal(received[2].headers['if-none-match'], '*');
    assert.equal(received[2].headers['content-type'], 'image/jpeg');
    assert.equal(received[2].headers.host, `127.0.0.1:${port}`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
    await new Promise((resolve) => server.close(resolve));
  }
});
