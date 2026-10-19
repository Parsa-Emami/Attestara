import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { webOptions } from '../../scripts/serve-web.mjs';

const cases = JSON.parse(await readFile(new URL('../fixtures/attestara/C32/cases.json', import.meta.url)));
const script = resolve('scripts/serve-web.mjs');
const config = resolve('ops/web/runtime-config.json');

async function availablePort() {
  const server = createServer();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const port = server.address().port;
  server.close();
  await once(server, 'close');
  return port;
}

async function launch(root, port) {
  const child = spawn(process.execPath, [script, '--root', root, '--config', config, '--port', String(port)], { stdio: ['ignore', 'pipe', 'pipe'] });
  let output = '';
  const started = new Promise((done, fail) => {
    const timeout = setTimeout(() => fail(new Error('web startup timed out')), 5000);
    child.stdout.on('data', chunk => {
      output += chunk;
      if (output.includes('"event":"started"')) { clearTimeout(timeout); done(); }
    });
    child.once('exit', code => { clearTimeout(timeout); fail(new Error(`web exited before startup: ${code}`)); });
  });
  try { await started; } catch (error) { child.kill(); throw error; }
  return child;
}

async function stop(child) {
  if (child.exitCode !== null) return;
  child.kill('SIGTERM');
  await once(child, 'exit');
}

test('C32 fixture is versioned and loopback input rejects nonlocal bind', () => {
  assert.equal(cases.schemaVersion, 1);
  assert.equal(cases.license, 'Apache-2.0');
  for (const host of cases.bindRejected) assert.throws(() => webOptions(['--host', host]), /loopback/);
  assert.equal(webOptions(['--host', '::1']).host, '::1');
});

test('C32 local HTTP boundaries survive restart', async t => {
  const dir = await mkdtemp(join(tmpdir(), 'attestara-c32-'));
  const root = join(dir, 'web');
  await mkdir(join(root, 'assets'), { recursive: true });
  await writeFile(join(root, 'index.html'), '<!doctype html><h1>local</h1>');
  await writeFile(join(root, cases.fingerprintedAsset.slice(1)), 'fingerprinted inert bytes');
  await writeFile(join(root, cases.unversionedAsset.slice(1)), 'unversioned inert bytes');
  const port = await availablePort();
  let child;
  t.after(async () => { if (child) await stop(child); await rm(dir, { recursive: true, force: true }); });
  const check = async () => {
    const base = `http://127.0.0.1:${port}`;
    for (const path of cases.noStore) {
      const response = await fetch(base + path, { headers: { Origin: cases.origin } });
      assert.equal(response.headers.get('cache-control'), 'no-store', path);
      assert.equal(response.headers.get('access-control-allow-origin'), null, path);
      assert.match(response.headers.get('content-security-policy'), /connect-src 'self'/, path);
    }
    assert.equal((await fetch(base + cases.fingerprintedAsset)).headers.get('cache-control'), 'public, max-age=31536000, immutable');
    assert.equal((await fetch(base + cases.unversionedAsset)).headers.get('cache-control'), 'no-store');
    assert.equal((await fetch(base + cases.disabledDomainRoute)).status, 503);
    assert.equal((await fetch(base + cases.disabledDomainRoute, { method: 'POST' })).status, 405);
    assert.equal((await fetch(base + '/package.json')).status, 404);
  };
  child = await launch(root, port);
  await check();
  await stop(child);
  child = await launch(root, port);
  await check();
});

test('C32 missing assets and configuration fail startup without raw path disclosure', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'attestara-c32-missing-'));
  try {
    const malformed = join(dir, 'malformed.json');
    await writeFile(malformed, '{invalid');
    for (const args of [['--root', dir, '--config', config], ['--root', dir, '--config', join(dir, 'missing.json')], ['--root', dir, '--config', malformed]]) {
      const result = spawnSync(process.execPath, [script, ...args, '--port', '43198'], { encoding: 'utf8', timeout: 5000 });
      assert.equal(result.status, 1);
      assert.match(result.stderr, /"event":"startup_failed"/);
      assert.doesNotMatch(result.stderr, new RegExp(dir));
    }
  } finally { await rm(dir, { recursive: true, force: true }); }
});
