import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, mkdtemp, mkdir, writeFile, access, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import http from 'node:http';
import { once } from 'node:events';
import { startLocal } from '../../scripts/start-local.mjs';
import { setup } from '../../scripts/oss-bootstrap.mjs';
import { validateEngineHealth } from '../../apps/web/src/lib/engine-health.mjs';
import { createWebServer } from '../../scripts/serve-web.mjs';
import { spawn, spawnSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { setTimeout as delay } from 'node:timers/promises';

const fixture = async name => {
 const value = JSON.parse(await readFile(new URL(`../fixtures/attestara/C03/${name}.json`, import.meta.url)));
 assert.equal(value.schemaVersion, 1); assert.equal(value.license, 'Apache-2.0'); assert.ok(value.provenance);
 return value;
};
const config = { projectId: 'attestara', apiBasePath: '/api', engineEnabled: false, maxPreviewBytes: 1048576 };

test('C03 rejects malformed health fixtures without asserting product readiness', async () => {
 const good = await fixture('positive');
 assert.equal(validateEngineHealth(good.health).scope, 'engine-process');
 for (const bad of (await fixture('negative')).health) assert.throws(() => validateEngineHealth(bad));
 assert.throws(() => validateEngineHealth({ ...good.health, secret: 'sentinel' }));
});

test('C03 setup invalidates stale output and fails explicitly for missing/mismatched Go or partial build', async t => {
 const dir = await mkdtemp(join(tmpdir(), 'attestara-setup-'));
 t.after(() => rm(dir, { recursive: true, force: true }));
 await mkdir(join(dir, 'dist/engine'), { recursive: true });
 const binary = join(dir, 'dist/engine/attestara');
 for (const probe of [{ error: new Error('ENOENT'), status: null }, { status: 0, stdout: 'go1.26.2\n' }]) {
  await writeFile(binary, 'stale');
  assert.throws(() => setup(['--skip-install'], () => probe, process.version, dir), /Go 1.27.2/);
  await assert.rejects(access(binary));
 }
 let calls = 0;
 assert.throws(() => setup(['--skip-install'], () => ++calls === 1 ? { status: 0, stdout: 'go1.27.2\n' } : { status: 1 }, process.version, dir), /Setup failed/);
 await assert.rejects(access(binary));
 assert.throws(() => setup(['--skip-install'], (command, args, options) => spawnSync(command, args, { ...options, env: { ...options.env, PATH: dir } }), process.version, dir), /Go 1.27.2/);
 for (const failure of ['build', 'probe']) {
  calls = 0;
  assert.throws(() => setup(['--skip-install'], () => {
   calls++;
   if (calls === 1) return { status: 0, stdout: 'go1.27.2\n' };
   if (calls === 2) return { status: 0 };
   if (calls === 4) { writeFileSync(binary + '.pending', 'partial'); return { status: failure === 'build' ? 1 : 0 }; }
   return { status: 0, stdout: '{"status":"ok"}' };
  }, process.version, dir), /Setup failed|Invalid engine/);
  await assert.rejects(access(binary)); await assert.rejects(access(binary + '.pending'));
 }
});

test('C03 real engine health, method boundaries, shutdown and restart', async () => {
 const expected = await fixture('expected');
 const dir = await mkdtemp(join(tmpdir(), 'attestara-no-execution-'));
 const marker = join(dir, 'NEVER_EXECUTED');
 try {
 for (let attempt = 0; attempt < 2; attempt++) {
  const local = await startLocal({ port: 0 });
  try {
   const base = `http://127.0.0.1:${local.server.address().port}`;
   const response = await fetch(base + '/api/engine/health');
   assert.equal(response.status, expected.success.healthStatus);
   assert.deepEqual(await response.json(), (await fixture('positive')).health);
   assert.equal((await fetch(base + '/api/run', { method: 'POST', body: `touch ${marker}` })).status, 405);
   await assert.rejects(access(marker));
   assert.equal((await fetch(base + '/api/run')).status, 503);
   assert.equal((await (await fetch(base + '/readyz')).json()).engineReady, false);
   assert.equal((await fetch(base)).status, 200);
  } finally { await local.close(); }
  assert.equal(await local.done, expected.success.shutdownExit);
  assert.notEqual(local.engine.exitCode, null);
 }
 } finally { await rm(dir, { recursive: true, force: true }); }
});

test('C03 occupied web port cleans up engine; crash stops web; cancellation stops engine', async () => {
 const occupied = http.createServer(); occupied.listen(0, '127.0.0.1'); await once(occupied, 'listening');
 try { await assert.rejects(startLocal({ port: occupied.address().port }), /EADDRINUSE/); }
 finally { await new Promise(resolve => occupied.close(resolve)); }
 const children = spawnSync('ps', ['--no-headers', '-o', 'comm=', '--ppid', String(process.pid)], { encoding: 'utf8' });
 assert.doesNotMatch(children.stdout, /attestara/);
 const crashed = await startLocal({ port: 0 });
 const url = `http://127.0.0.1:${crashed.server.address().port}`;
 crashed.engine.kill('SIGKILL');
 assert.equal(await crashed.done, 1);
 await assert.rejects(fetch(url));
 const controller = new AbortController();
 const cancelled = await startLocal({ port: 0, signal: controller.signal });
 controller.abort();
 assert.equal(await cancelled.done, 0);
 assert.notEqual(cancelled.engine.exitCode, null);
 const early = new AbortController(); early.abort();
 await assert.rejects(startLocal({ port: 0, signal: early.signal }), /cancel/i);
});

test('C03 actual npm start handles terminal signals and engine crash without orphans', { timeout: 20000 }, async () => {
 for (const action of ['SIGINT', 'SIGTERM', 'crash']) {
  const reservation = http.createServer(); reservation.listen(0, '127.0.0.1'); await once(reservation, 'listening');
  const port = reservation.address().port;
  await new Promise(resolve => reservation.close(resolve));
  const child = spawn('npm', ['start', '--', '--port', String(port)], { detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  child.stderr.resume();
  const ended = new Promise(resolve => child.once('close', (code, signal) => resolve({ code, signal })));
  try {
   const event = await new Promise((resolve, reject) => {
    let text = '';
    const timer = setTimeout(() => reject(new Error('npm start deadline')), 5000);
    child.once('error', error => { clearTimeout(timer); reject(error); });
    ended.then(() => { clearTimeout(timer); reject(new Error('npm start exited before readiness')); });
    child.stdout.on('data', data => {
     text += data;
     if (text.length > 8192) { clearTimeout(timer); reject(new Error('Unexpected startup output')); return; }
     for (const line of text.split('\n')) if (line.startsWith('{') && line.endsWith('}')) {
      const value = JSON.parse(line);
      if (value.service === 'attestara-local') { clearTimeout(timer); resolve(value); }
     }
    });
   });
   if (action === 'crash') process.kill(event.enginePid, 'SIGKILL');
   else process.kill(-child.pid, action);
   const result = await Promise.race([ended, delay(5000, undefined, { ref: false }).then(() => { throw new Error('npm shutdown deadline'); })]);
   if (action === 'crash') assert.equal(result.code, 1);
   else assert.ok(result.code === 0 || result.signal === action || result.code === (action === 'SIGINT' ? 130 : 143));
   let alive = true;
   for (let attempt = 0; attempt < 100 && alive; attempt++) {
    try { process.kill(event.enginePid, 0); await delay(20); } catch (error) { if (error.code !== 'ESRCH') throw error; alive = false; }
   }
   assert.equal(alive, false, 'engine survived terminal shutdown');
   await assert.rejects(fetch(`http://127.0.0.1:${port}/healthz`));
  } finally { try { process.kill(-child.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; } }
 }
});

test('C03 same-origin boundary rejects unavailable, malformed, redirected and oversized upstream health', async t => {
 let body = '{}'; let redirect = false; let stalled = false;
 const upstream = http.createServer((req, res) => { if (stalled) res.write('{'); else if (redirect) { res.writeHead(302, { location: 'http://outside.invalid/' }); res.end(); } else res.end(body); });
 upstream.listen(0, '127.0.0.1'); await once(upstream, 'listening');
 const url = `http://127.0.0.1:${upstream.address().port}/healthz`;
 const server = createWebServer({ root: 'dist/web', config, projectId: 'attestara', engineURL: url });
 server.listen(0, '127.0.0.1'); await once(server, 'listening');
 t.after(async () => { upstream.closeAllConnections(); server.closeAllConnections(); await Promise.all([new Promise(r => upstream.close(r)), new Promise(r => server.close(r))]); });
 const healthURL = `http://127.0.0.1:${server.address().port}/api/engine/health`;
 for (body of ['{}', '{', 'x'.repeat(5000)]) {
  const res = await fetch(healthURL); assert.equal(res.status, 503); assert.deepEqual(await res.json(), { error: 'engine_unavailable' });
 }
 redirect = true;
 assert.equal((await fetch(healthURL)).status, 503);
 stalled = true;
 assert.equal((await fetch(healthURL)).status, 503);
 assert.throws(() => createWebServer({ root: 'dist/web', config, projectId: 'attestara', engineURL: 'http://outside.invalid/healthz' }), /loopback/);
});
