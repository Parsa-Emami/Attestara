import http from 'node:http';
import { readFile, realpath, stat } from 'node:fs/promises';
import { resolve, relative, extname, isAbsolute } from 'node:path';
import { pathToFileURL } from 'node:url';
import { randomUUID } from 'node:crypto';
import { validateRuntimeConfig } from '../apps/web/src/lib/runtime-config.mjs';
import { validateEngineHealth } from '../apps/web/src/lib/engine-health.mjs';

// Only fixed states enter diagnostics; raw errors, child output and request data never do.
export function runtimeDiagnostic(entryPoint, event, runId) {
  const ready = event === 'started';
  return {
    schemaVersion: 1, service: entryPoint === 'local' ? 'attestara-local' : 'attestara-web',
    entryPoint, runId, event, coreReady: false,
    readiness: {
      web: ready ? 'ready' : 'not_ready',
      engine: entryPoint === 'web' ? 'not_configured' : ready ? 'process_healthy' : 'unavailable',
      storage: entryPoint === 'local' && ready ? 'ready' : entryPoint === 'local' ? 'unavailable' : 'not_integrated', providers: entryPoint === 'local' ? 'not_required' : 'not_integrated', requiredProviders: [],
    },
  };
}

export async function probeWebAssets(root) {
  const publicRoot = resolve(root);
  const index = await realpath(resolve(publicRoot, 'index.html'));
  const diff = relative(publicRoot, index);
  if (diff === '..' || diff.startsWith('../') || isAbsolute(diff) || !(await stat(index)).isFile()) throw new Error('Web assets unavailable.');
}

export function validateEngineURL(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || !['127.0.0.1', '[::1]'].includes(url.hostname) || !url.port || url.username || url.password || url.pathname !== '/healthz' || url.search || url.hash) throw new Error('Engine health must use an explicit loopback HTTP port.');
  return url.href;
}

async function boundedJSON(response, limit) {
  if (!response.body) throw new Error('Missing response body.');
  const reader = response.body.getReader();
  const chunks = []; let size = 0;
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.length;
      if (size > limit) throw new Error('Response exceeds limit.');
      chunks.push(value);
    }
  } finally { await reader.cancel(); }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

export async function probeEngineHealth(url) {
  const response = await fetch(validateEngineURL(url), { signal: AbortSignal.timeout(1500), redirect: 'error', credentials: 'omit' });
  if (!response.ok) throw new Error('Engine unavailable.');
  return validateEngineHealth(await boundedJSON(response, 4096));
}

export async function probeSnapshotReadiness(url) {
  const response = await fetch(new URL('/readyz', validateEngineURL(url)), { signal: AbortSignal.timeout(1500), redirect: 'error', credentials: 'omit' });
  const state = await boundedJSON(response, 1024);
  if (state.schema_version !== '1' || !['ready', 'locked', 'corrupt', 'unavailable'].includes(state.storage) || state.scope !== 'local-cli-snapshots' || state.policy !== 'builtin' || !Array.isArray(state.required_providers) || state.required_providers.length !== 0 || response.ok !== (state.storage === 'ready')) throw new Error('Invalid snapshot readiness.');
  return state.storage;
}

const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8', '.svg': 'image/svg+xml', '.png': 'image/png', '.webmanifest': 'application/manifest+json' };
const csp = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'";
export function createWebServer({ root, config, projectId, engineURL, snapshotReadiness = false }) {
  const valid = validateRuntimeConfig(config, projectId);
  if (engineURL !== undefined) engineURL = validateEngineURL(engineURL);
  const publicRoot = resolve(root);
  const within = path => { const diff = relative(publicRoot, path); return diff !== '..' && !diff.startsWith('../') && !isAbsolute(diff); };
  return http.createServer({ requestTimeout: 15000, headersTimeout: 10000, keepAliveTimeout: 5000, maxHeaderSize: 16384 }, async (req, res) => {
    res.setHeader('Content-Security-Policy', csp);
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    res.setHeader('Permissions-Policy', 'camera=(), microphone=(), geolocation=()');
    res.setHeader('X-Frame-Options', 'DENY');
    res.setHeader('Cache-Control', 'no-store');
    const send = (status, body, type = 'application/json; charset=utf-8') => {
      res.writeHead(status, { 'Content-Type': type, 'Content-Length': Buffer.byteLength(body) });
      res.end(req.method === 'HEAD' ? undefined : body);
    };
    if (!['GET', 'HEAD'].includes(req.method ?? '')) { res.setHeader('Allow', 'GET, HEAD'); send(405, '{"error":"method_not_allowed"}'); return; }
    let path;
    try {
      path = decodeURIComponent((req.url ?? '/').split('?')[0]);
      if (!path.startsWith('/') || path.includes('\\') || path.includes('\0') || path.split('/').some(part => part.startsWith('.'))) throw new Error();
    } catch { send(400, '{"error":"invalid_path"}'); return; }
    if (path === '/healthz') { send(200, '{"status":"ok","scope":"web-shell"}'); return; }
    if (path === '/readyz' && snapshotReadiness) {
      try {
        await probeWebAssets(publicRoot);
        await probeEngineHealth(engineURL);
        const storage = await probeSnapshotReadiness(engineURL);
        send(storage === 'ready' ? 200 : 503, JSON.stringify({status: storage === 'ready' ? 'ready' : 'not_ready', scope: 'local-cli-snapshots', engineReady: true, storage, policy: 'builtin', requiredProviders: []}));
      } catch { send(503, '{"status":"not_ready","scope":"local-cli-snapshots"}'); }
      return;
    }
    if (path === '/readyz') {
      try { await probeWebAssets(publicRoot); send(200, '{"status":"ready","scope":"web-shell","engineReady":false}'); }
      catch { send(503, '{"status":"not_ready","scope":"web-shell"}'); }
      return;
    }
    if (path === '/api/engine/health') {
      try {
        if (!engineURL) throw new Error('No local engine.');
        send(200, JSON.stringify(await probeEngineHealth(engineURL)));
      } catch { send(503, '{"error":"engine_unavailable"}'); }
      return;
    }
    if (path === '/api' || path.startsWith('/api/')) { send(503, '{"error":"engine_not_integrated"}'); return; }
    if (path === '/runtime-config.json') { send(200, JSON.stringify(valid)); return; }
    const asset = path.startsWith('/assets/');
    if (!(asset || ['/', '/index.html', '/icon.svg', '/manifest.webmanifest', '/robots.txt'].includes(path))) { send(404, '{"error":"not_found"}'); return; }
    try {
      const filename = await realpath(resolve(publicRoot, '.' + (path === '/' ? '/index.html' : path)));
      if (!within(filename) || !(await stat(filename)).isFile()) { send(404, '{"error":"not_found"}'); return; }
      const bytes = await readFile(filename);
      const fingerprinted = asset && /-[A-Za-z0-9_-]{8,}\.[^.]+$/.test(path);
      res.writeHead(200, { 'Content-Type': types[extname(filename)] ?? 'application/octet-stream', 'Content-Length': bytes.length, 'Cache-Control': fingerprinted ? 'public, max-age=31536000, immutable' : 'no-store' });
      res.end(req.method === 'HEAD' ? undefined : bytes);
    } catch { send(404, '{"error":"not_found"}'); }
  });
}

export function webOptions(args) {
  const options = { host: '127.0.0.1', port: '4173', root: 'dist/web', config: 'ops/web/runtime-config.json', store: '.local-data/attestara' };
  for (let i = 0; i < args.length; i += 2) {
    const key = args[i].slice(2);
    if (!args[i].startsWith('--') || !Object.hasOwn(options, key) || !args[i + 1]) throw new Error('Usage: node scripts/serve-web.mjs [--host 127.0.0.1] [--port 4173] [--root dist/web] [--config path]');
    options[key] = args[i + 1];
  }
  if (!['127.0.0.1', '::1'].includes(options.host)) throw new Error('Bind to loopback and use the documented TLS proxy.');
  if (!/^\d+$/.test(options.port) || +options.port < 1 || +options.port > 65535) throw new Error('Invalid port.');
  return { ...options, port: +options.port };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const runId = randomUUID();
  try {
    const options = webOptions(process.argv.slice(2));
    const product = JSON.parse(await readFile(new URL('../ops/web/brand.json', import.meta.url), 'utf8'));
    const config = JSON.parse(await readFile(resolve(options.config), 'utf8'));
    await probeWebAssets(options.root);
    const server = createWebServer({ root: options.root, config, projectId: product.slug });
    server.maxRequestsPerSocket = 1000;
    let finished = false;
    const finish = (event, code) => {
      if (finished) return;
      finished = true;
      (code ? console.error : console.log)(JSON.stringify(runtimeDiagnostic('web', event, runId)));
      process.exitCode = code;
    };
    server.once('error', () => { finish(server.listening ? 'runtime_failed' : 'startup_failed', 1); server.closeAllConnections(); server.close(); });
    server.listen(+options.port, options.host, () => console.log(JSON.stringify({ ...runtimeDiagnostic('web', 'started', runId), host: options.host, port: options.port, engineReady: false })));
    let stopping = false;
    for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => {
      if (stopping) return;
      stopping = true;
      server.close(() => { finish('stopped', 0); process.exit(0); });
      setTimeout(() => { finish('runtime_failed', 1); server.closeAllConnections(); process.exit(1); }, 10000).unref();
    });
  } catch { console.error(JSON.stringify(runtimeDiagnostic('web', 'startup_failed', runId))); process.exitCode = 1; }
}
