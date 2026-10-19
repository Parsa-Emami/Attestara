import { spawn } from 'node:child_process';
import { readFile, stat } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { randomUUID } from 'node:crypto';
import { createWebServer, probeEngineHealth, probeSnapshotReadiness, probeWebAssets, validateEngineURL, webOptions, runtimeDiagnostic } from './serve-web.mjs';

const repository = fileURLToPath(new URL('../', import.meta.url));

export async function startLocal({ host = '127.0.0.1', port = 4173, root = resolve(repository, 'dist/web'), config: configPath = resolve(repository, 'ops/web/runtime-config.json'), store = resolve(repository, '.local-data/attestara'), signal } = {}) {
  if (signal?.aborted) throw new Error('Local startup cancelled.');
  if (!['127.0.0.1', '::1'].includes(host) || !Number.isInteger(port) || port < 0 || port > 65535) throw new Error('Invalid local listen address.');
  const config = JSON.parse(await readFile(configPath, 'utf8'));
  await probeWebAssets(root);
  const binary = resolve(repository, 'dist/engine/attestara');
  try { await stat(binary); } catch { throw new Error('Engine is not prepared; run npm run setup.'); }
  // Spawn the repository-built executable directly; no shell or user payload is accepted.
  // https://nodejs.org/docs/latest-v24.x/api/child_process.html#child_processspawncommand-args-options
  // Discard raw stderr; the CLI emits fixed lifecycle diagnostics instead.
  const engine = spawn(binary, ['--listen', '127.0.0.1:0', '--store', resolve(store)], { cwd: repository, stdio: ['ignore', 'pipe', 'ignore'] });
  let server; let stopping = false; let closing;
  let finish;
  const done = new Promise(resolve => { finish = resolve; });
  const exited = new Promise(resolve => { engine.once('close', resolve); });
  async function close(code = 0) {
    if (closing) return closing;
    stopping = true;
    signal?.removeEventListener('abort', abort);
    closing = (async () => {
      const webClosed = new Promise(resolve => {
        if (!server?.listening) { resolve(); return; }
        server.close(resolve); server.closeAllConnections();
      });
      if (engine.exitCode === null && engine.signalCode === null) engine.kill('SIGTERM');
      let forced = false;
      const timer = setTimeout(() => { forced = true; engine.kill('SIGKILL'); }, 4000);
      await Promise.all([exited, webClosed]); clearTimeout(timer);
      const result = forced || (engine.exitCode !== null && engine.exitCode !== 0) ? 1 : code;
      finish(result);
      return result;
    })();
    return closing;
  }
  const abort = () => { void close(); };
  signal?.addEventListener('abort', abort, { once: true });
  engine.once('error', () => { void close(1); });
  engine.once('exit', () => { if (!stopping) void close(1); });
  try {
    const engineURL = await new Promise((accept, reject) => {
      let bytes = '';
      const timer = setTimeout(() => fail(new Error('Engine startup deadline exceeded.')), 5000);
      const cleanup = () => { clearTimeout(timer); engine.stdout.off('data', data); engine.off('error', fail); engine.off('exit', failedExit); signal?.removeEventListener('abort', cancelled); };
      const fail = error => { cleanup(); reject(error); };
      const failedExit = () => fail(new Error('Engine exited during startup.'));
      const cancelled = () => fail(new Error('Local startup cancelled.'));
      const data = chunk => {
        if (Buffer.byteLength(bytes) + chunk.length > 4096) { fail(new Error('Invalid engine startup response.')); return; }
        bytes += chunk.toString('utf8');
        if (!bytes.includes('\n')) return;
        try {
          const event = JSON.parse(bytes.slice(0, bytes.indexOf('\n')));
          if (event.event !== 'listening' || typeof event.address !== 'string') throw new Error('Invalid engine startup response.');
          const url = validateEngineURL('http://' + event.address + '/healthz');
          cleanup(); engine.stdout.resume(); accept(url);
        } catch (error) { fail(error); }
      };
      engine.stdout.on('data', data); engine.once('error', fail); engine.once('exit', failedExit); signal?.addEventListener('abort', cancelled, { once: true });
      if (signal?.aborted) cancelled();
    });
    await probeEngineHealth(engineURL);
    if (await probeSnapshotReadiness(engineURL) !== 'ready') throw new Error('Snapshot storage unavailable.');
    if (stopping) throw new Error('Local startup cancelled or engine exited.');
    server = createWebServer({ root, config, projectId: 'attestara', engineURL, snapshotReadiness: true });
    server.maxRequestsPerSocket = 1000;
    await new Promise((accept, reject) => {
      server.once('error', reject);
      server.listen(port, host, () => { server.off('error', reject); accept(); });
    });
    server.on('error', () => { void close(1); });
    if (stopping) { server.closeAllConnections(); await new Promise(resolve => server.close(resolve)); throw new Error('Local startup cancelled.'); }
    return { server, engine, engineURL, close, done };
  } catch (error) { await close(1); throw error; }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const runId = randomUUID();
  const controller = new AbortController();
  for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => controller.abort());
  try {
    const local = await startLocal({ ...webOptions(process.argv.slice(2)), signal: controller.signal });
    console.log(JSON.stringify({ ...runtimeDiagnostic('local', 'started', runId), host: local.server.address().address, port: local.server.address().port, enginePid: local.engine.pid }));
    process.exitCode = await local.done;
    (process.exitCode ? console.error : console.log)(JSON.stringify(runtimeDiagnostic('local', process.exitCode ? 'runtime_failed' : 'stopped', runId)));
  } catch {
    process.exitCode = controller.signal.aborted ? 0 : 1;
    (process.exitCode ? console.error : console.log)(JSON.stringify(runtimeDiagnostic('local', controller.signal.aborted ? 'stopped' : 'startup_failed', runId)));
  }
}
