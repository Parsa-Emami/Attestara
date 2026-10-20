import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createServer } from 'node:net';

// Both local packaging and published installation exercise the same restricted launcher.
export function installedRuntime() {
  let service;
  async function stop() {
    if (!service) return;
    const child = service; service = undefined;
    if (child.exitCode !== null || child.signalCode !== null) return;
    const ended = once(child, 'close'); child.kill('SIGTERM');
    const timer = setTimeout(() => child.kill('SIGKILL'), 6000);
    const [code] = await ended; clearTimeout(timer); assert.equal(code, 0);
  }
  async function start(runtime, store) {
    const reservation = createServer();
    await new Promise(accept => reservation.listen(0, '127.0.0.1', accept));
    const port = reservation.address().port;
    await new Promise(accept => reservation.close(accept));
    service = spawn(process.execPath, ['scripts/start-local.mjs', '--port', String(port), '--store', store], { cwd: runtime, env: { ...process.env, PATH: '/nonexistent' }, stdio: ['ignore', 'pipe', 'pipe'] });
    let text = '', errors = '';
    service.stderr.on('data', bytes => { errors += bytes; });
    service.on('close', code => { if (code) console.error('Packaged launcher failure:', text, errors); });
    const event = await new Promise((accept, reject) => {
      const timer = setTimeout(() => reject(new Error('Packaged startup deadline')), 15000);
      service.once('exit', code => { clearTimeout(timer); reject(new Error('Packaged launcher failed: ' + code)); });
      service.stdout.on('data', bytes => {
        text += bytes;
        if (text.includes('\n')) { clearTimeout(timer); try { accept(JSON.parse(text.split('\n')[0])); } catch (error) { reject(error); } }
      });
    });
    assert.equal(event.event, 'started');
    const url = 'http://127.0.0.1:' + event.port;
    assert.equal((await fetch(url + '/readyz')).status, 200);
    assert.equal((await fetch(url + '/api/not-enabled')).status, 503);
    return url;
  }
  return { start, stop };
}
