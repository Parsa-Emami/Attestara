import test from 'node:test';
import { randomUUID } from 'node:crypto';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import http from 'node:http';
import { mkdtemp, mkdir, readFile, copyFile, writeFile, chmod, access, rm, symlink } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { setTimeout as delay } from 'node:timers/promises';
import { createWebServer } from '../../scripts/serve-web.mjs';
import { startLocal } from '../../scripts/start-local.mjs';

const expected = JSON.parse(await readFile(new URL('../fixtures/attestara/C29/expected.json', import.meta.url)));
const repository = new URL('../../', import.meta.url);
const read = path => readFile(new URL(path, repository));

async function freePort() {
  const server = http.createServer(); server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const port = server.address().port;
  await new Promise(resolve => server.close(resolve));
  return port;
}

function launch(t, script, args = [], cwd = repository, env = process.env) {
  const store = join(tmpdir(), 'attestara-c30-lifecycle-' + randomUUID());
  if (script.endsWith('start-local.mjs')) args = [...args, '--store', store];
  const child = spawn(process.execPath, [script, ...args], { cwd, env, stdio: ['ignore', 'pipe', 'pipe'] });
  let stdout = '', stderr = '', overflow = false;
  let first;
  const started = new Promise(resolve => { first = resolve; });
  const timer = setTimeout(() => child.kill('SIGKILL'), 8000);
  child.stdout.on('data', bytes => {
    stdout += bytes.toString();
    if (stdout.length > 8192) { overflow = true; stdout = stdout.slice(0, 8193); child.kill('SIGKILL'); }
    if (stdout.includes('\n')) first(JSON.parse(stdout.split('\n')[0]));
  });
  child.stderr.on('data', bytes => {
    stderr += bytes.toString();
    if (stderr.length > 8192) { overflow = true; stderr = stderr.slice(0, 8193); child.kill('SIGKILL'); }
  });
  const done = new Promise(resolve => child.once('close', (code, signal) => { clearTimeout(timer); first(undefined); resolve({code, signal}); }));
  t.after(async () => { child.kill('SIGKILL'); await done; await rm(store, {recursive:true,force:true}); });
  return { child, started, done, output: () => ({stdout, stderr, overflow}) };
}

function diagnostics(run) {
  const {stdout, stderr, overflow} = run.output();
  assert.equal(overflow, false);
  for (const sentinel of expected.sentinels) assert.equal((stdout + stderr).includes(sentinel), false);
  const lines = (stdout + stderr).trim().split('\n').filter(Boolean);
  assert.ok(lines.length <= expected.maxEventsPerRun);
  const events = lines.map(line => { assert.ok(Buffer.byteLength(line) <= expected.maxLineBytes); return JSON.parse(line); });
  assert.ok(events.length > 0);
  for (const event of events) {
    assert.equal(event.schemaVersion, expected.schemaVersion);
    assert.match(event.runId, /^[0-9a-f-]{36}$/);
    assert.equal(event.runId, events[0].runId);
    assert.equal(event.coreReady, false);
    assert.equal(event.readiness.storage, event.entryPoint === 'local' ? event.event === 'started' ? 'ready' : 'unavailable' : 'not_integrated');
    assert.equal(event.readiness.providers, event.entryPoint === 'local' ? 'not_required' : 'not_integrated');
    assert.deepEqual(event.readiness.requiredProviders, []);
  }
  return events;
}

test('C29 real Go lifecycle reports separate readiness, restarts and fails visibly without executing source', async t => {
  const dir = await mkdtemp(join(tmpdir(), 'attestara-c29-marker-'));
  t.after(() => rm(dir, {recursive:true,force:true}));
  const marker = join(dir, 'NEVER_EXECUTED');
  let previousRun;
  for (const action of ['SIGTERM', 'crash']) {
    const port = await freePort();
    const run = launch(t, 'scripts/start-local.mjs', ['--port', String(port)]);
    const event = await run.started;
    assert.equal(event?.event, 'started');
    assert.deepEqual(event.readiness, {...expected.started, storage:'ready', providers:'not_required'});
    assert.notEqual(event.runId, previousRun); previousRun = event.runId;
    const base = `http://127.0.0.1:${port}`;
    assert.deepEqual(await (await fetch(base + '/healthz')).json(), {status:'ok',scope:'web-shell'});
    const ready = await (await fetch(base + '/readyz')).json();
    assert.equal(ready.scope, 'local-cli-snapshots'); assert.equal(ready.engineReady, true); assert.equal(ready.storage, 'ready');
    const health = await fetch(base + '/api/engine/health'); assert.equal(health.status, 200);
    assert.equal((await health.json()).scope, 'engine-process');
    assert.equal((await fetch(base + '/api/run', {method:'POST',body:`touch ${marker}`})).status, 405);
    assert.equal((await fetch(base + '/api/diagnostics')).status, 503);
    await assert.rejects(access(marker));
    if (action === 'crash') process.kill(event.enginePid, 'SIGKILL'); else run.child.kill(action);
    assert.equal((await run.done).code, action === 'crash' ? 1 : 0);
    const events = diagnostics(run); assert.equal(events.length, 2);
    assert.equal(events[1].event, action === 'crash' ? 'runtime_failed' : 'stopped');
    assert.deepEqual(events[1].readiness, {...expected.failed, storage: 'unavailable', providers: 'not_required'});
    await assert.rejects(fetch(base + '/healthz'));
    assert.throws(() => process.kill(event.enginePid, 0), {code:'ESRCH'});
  }
});

async function temporaryCheckout(t) {
  const dir = await mkdtemp(join(tmpdir(), 'attestara-c29-checkout-'));
  t.after(() => rm(dir, {recursive:true,force:true}));
  for (const path of ['scripts', 'apps/web/src/lib', 'ops/web', 'dist/web', 'dist/engine']) await mkdir(join(dir, path), {recursive:true});
  for (const path of ['scripts/start-local.mjs','scripts/serve-web.mjs','apps/web/src/lib/runtime-config.mjs','apps/web/src/lib/engine-health.mjs','ops/web/runtime-config.json','ops/web/brand.json']) await writeFile(join(dir,path), await read(path));
  await writeFile(join(dir,'dist/web/index.html'), '<!doctype html><h1>C29 synthetic shell</h1>');
  return dir;
}

test('C29 missing prerequisites, secret-bearing configuration and occupied port emit bounded safe diagnostics', async t => {
  const dir = await temporaryCheckout(t);
  for (const [script,args] of [
    ['scripts/start-local.mjs',[]],
    ['scripts/serve-web.mjs',['--config','/private/C29_PATH_SENTINEL']],
    ['scripts/start-local.mjs',['--config','/private/C29_PATH_SENTINEL']],
    ['scripts/serve-web.mjs',['--root','/private/C29_PATH_SENTINEL']],
    ['scripts/start-local.mjs',['--root','/private/C29_PATH_SENTINEL']],
  ]) {
    const run = launch(t,script,args,dir);
    assert.equal((await run.done).code, 1);
    assert.equal(diagnostics(run)[0].event, 'startup_failed');
  }
  await writeFile(join(dir,'ops/web/runtime-config.json'), '{"token":"C29_SECRET_SENTINEL"}');
  const invalid = launch(t,'scripts/serve-web.mjs',[],dir);
  assert.equal((await invalid.done).code,1); assert.equal(diagnostics(invalid)[0].event,'startup_failed');
  const occupied = http.createServer(); occupied.listen(0,'127.0.0.1'); await once(occupied,'listening');
  try {
    for (const script of ['scripts/start-local.mjs','scripts/serve-web.mjs']) {
      const run = launch(t,script,['--port',String(occupied.address().port)]);
      assert.equal((await run.done).code,1); assert.equal(diagnostics(run)[0].event,'startup_failed');
    }
  } finally { await new Promise(resolve => occupied.close(resolve)); }
});

test('C29 hostile child stdout/stderr and malformed health never enter operator logs', async t => {
  const dir = await temporaryCheckout(t);
  await copyFile(new URL('../fixtures/attestara/C29/untrusted-engine.mjs',import.meta.url),join(dir,'dist/engine/attestara'));
  await chmod(join(dir,'dist/engine/attestara'),0o700);
  for (const mode of ['malformed','oversized','invalid_health','noisy_health']) {
    const run = launch(t,'scripts/start-local.mjs',['--port',String(await freePort())],dir,{...process.env,ATTESTARA_C29_FIXTURE:mode});
    // C30 also requires an independent bounded storage probe. A hostile child
    // with valid process health alone must no longer satisfy startup readiness.
    assert.equal((await run.done).code,1);
    assert.equal(diagnostics(run)[0].event,'startup_failed');
  }
});

test('C29 standalone web stays web-only and unavailable engine health fails closed', async t => {
  const port = await freePort();
  const run = launch(t,'scripts/serve-web.mjs',['--port',String(port)]);
  const event = await run.started;
  assert.equal(event?.event,'started'); assert.equal(event.readiness.engine,'not_configured');
  const base = `http://127.0.0.1:${port}`;
  const health = await fetch(base + '/api/engine/health'); assert.equal(health.status,503);
  assert.deepEqual(await health.json(),{error:'engine_unavailable'});
  assert.equal((await fetch(base + '/readyz')).status,200);
  run.child.kill('SIGTERM'); assert.equal((await run.done).code,0); diagnostics(run);
  assert.equal(JSON.parse(await read('ops/web/runtime-config.json')).engineEnabled,false);
  assert.deepEqual(await read('ops/web/runtime-config.json'), await read('apps/web/public/runtime-config.json'));
});

test('C29 directory or escaping index cannot satisfy web readiness or launcher startup', async t => {
  const dir = await temporaryCheckout(t);
  const root = join(dir,'dist/web');
  const config = JSON.parse(await read('ops/web/runtime-config.json'));
  const server = createWebServer({root,config,projectId:'attestara'});
  server.listen(0,'127.0.0.1'); await once(server,'listening');
  t.after(async () => { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); });
  const base = `http://127.0.0.1:${server.address().port}`;
  for (const kind of ['directory','escaping_symlink']) {
    await rm(join(root,'index.html'),{recursive:true,force:true});
    if (kind === 'directory') await mkdir(join(root,'index.html'));
    else { await writeFile(join(dir,'private.html'),'C29_SECRET_SENTINEL'); await symlink(join(dir,'private.html'),join(root,'index.html')); }
    assert.equal((await fetch(base + '/healthz')).status,200);
    const response = await fetch(base + '/readyz'); assert.equal(response.status,503);
    assert.deepEqual(await response.json(),{status:'not_ready',scope:'web-shell'});
    await assert.rejects(startLocal({root,port:0}), /Web assets unavailable/);
    for (const script of ['scripts/start-local.mjs','scripts/serve-web.mjs']) {
      const run = launch(t,script,[],dir);
      assert.equal((await run.done).code,1); assert.equal(diagnostics(run)[0].event,'startup_failed');
    }
  }
});
