const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');
const { EventEmitter } = require('node:events');

for (const platform of ['darwin', 'linux', 'win32']) {
  test(`packaged ${platform} overrides inherited network binding`, () => {
    let spawned;
    const child = new EventEmitter();
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    const context = {
      require(name) {
        if (name === 'electron') return { app: {
          getPath: () => '/isolated-user',
          whenReady: () => ({ then() {} }), on() {},
        } };
        if (name === 'child_process') return { spawn(binary, args, options) {
          spawned = { binary, args, options }; return child;
        } };
        if (name === 'fs') return { existsSync: () => true };
        return require(name);
      },
      process: { env: { LISTEN_HOST: '0.0.0.0', PORT: '9999', KEEP_ME: 'control' }, platform, resourcesPath: '/bundle' },
      console: { log() {}, error() {} },
      __dirname: __dirname,
      setTimeout() {}, clearTimeout() {},
    };
    vm.createContext(context);
    vm.runInContext(fs.readFileSync(path.join(__dirname, 'main.js'), 'utf8'), context);
    vm.runInContext('startServer()', context);
    assert.ok(spawned);
    assert.equal(spawned.options.env.LISTEN_HOST, '127.0.0.1');
    assert.equal(spawned.options.env.PORT, '3000');
    assert.equal(spawned.options.env.KEEP_ME, 'control');
    assert.equal(spawned.options.env.SQLITE_PATH, '/isolated-user/data/omnimux.db');
    assert.equal(spawned.options.cwd, '/bundle');
    assert.equal(spawned.binary, `/bundle/bin/${platform === 'win32' ? 'omnimux.exe' : 'omnimux'}`);
  });
}
