import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { createWebServer } from '../../scripts/serve-web.mjs';

test('real isolated proxy preserves web boundaries and fails when upstream is down', { timeout: 15000 }, async t => {
  const nginx = process.env.NGINX_BIN || 'nginx';
  assert.equal(spawnSync(nginx, ['-v']).status, 0, 'Install Nginx or set NGINX_BIN for the real proxy regression.');
  const root = await mkdtemp(join(tmpdir(), 'attestara-c34-nginx-'));
  let upstream, reservation, proxy;
  t.after(async () => {
    if (proxy && proxy.exitCode === null && proxy.signalCode === null) {
      const closed = once(proxy, 'close'); proxy.kill('SIGTERM'); await closed;
    }
    for (const server of [upstream, reservation]) if (server?.listening) {
      const closed = once(server, 'close'); server.close(); server.closeAllConnections?.(); await closed;
    }
    await rm(root, { recursive: true, force: true });
  });
  for (const dir of ['logs', 'temp', 'web']) await mkdir(join(root, dir));
  await writeFile(join(root, 'web/index.html'), '<!doctype html><h1>Synthetic local proxy fixture</h1>');
  const config = JSON.parse(await readFile(new URL('../../ops/web/runtime-config.json', import.meta.url)));
  upstream = createWebServer({ root: join(root, 'web'), config, projectId: config.projectId });
  upstream.listen(0, '127.0.0.1'); await once(upstream, 'listening');
  let observed;
  upstream.prependListener('request', request => { observed = request.headers; });
  reservation = createServer();
  reservation.listen(0, '127.0.0.1'); await once(reservation, 'listening');
  const port = reservation.address().port;
  const template = await readFile(new URL('../../ops/web/nginx.local.conf', import.meta.url), 'utf8');
  assert.match(template, /listen 127\.0\.0\.1:8080;/);
  assert.doesNotMatch(template, /^\s*(?:include|listen\s+(?:[^;]*:)?80\s*;)/m);
  const path = join(root, 'nginx.conf');
  await writeFile(path, template.replace('127.0.0.1:8080', `127.0.0.1:${port}`).replace('127.0.0.1:4173', `127.0.0.1:${upstream.address().port}`));
  const args = ['-p', root + '/', '-c', path];
  // The port is deliberately occupied here: syntax success does not prove a free listener.
  const syntax = spawnSync(nginx, [...args, '-t'], { encoding: 'utf8' });
  assert.equal(syntax.status, 0, syntax.stderr);
  reservation.close(); await once(reservation, 'close');
  proxy = spawn(nginx, [...args, '-g', 'daemon off; master_process off;'], { stdio: 'ignore' });
  const base = `http://127.0.0.1:${port}`;
  let ready = false;
  for (let attempt = 0; attempt < 50 && !ready; attempt++) {
    try { ready = (await fetch(base + '/healthz', { signal: AbortSignal.timeout(1000) })).ok; }
    catch { await delay(50); }
    if (proxy.exitCode !== null) break;
  }
  assert.ok(ready, 'Proxy must actually start; nginx -t is insufficient.');
  const page = await fetch(base, { headers: { 'X-Forwarded-For': '203.0.113.7', 'X-Forwarded-Proto': 'https' } });
  assert.equal(page.status, 200);
  assert.equal(page.headers.get('cache-control'), 'no-store');
  assert.equal(page.headers.get('x-frame-options'), 'DENY');
  assert.match(page.headers.get('content-security-policy'), /connect-src 'self'/);
  assert.equal(observed.host, `127.0.0.1:${port}`);
  assert.equal(observed['x-forwarded-for'], '127.0.0.1');
  assert.equal(observed['x-forwarded-proto'], 'http');
  assert.equal((await fetch(base + '/api/releases')).status, 503);
  assert.equal((await fetch(base + '/api/releases', { method: 'POST' })).status, 405);
  const closed = once(upstream, 'close'); upstream.close(); upstream.closeAllConnections(); await closed;
  assert.equal((await fetch(base + '/healthz')).status, 502);
});
