import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, rm, writeFile, cp, readFile, realpath, statfs } from 'node:fs/promises';
import { tmpdir, platform, arch, release, cpus, totalmem } from 'node:os';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import http from 'node:http';
import { createHash } from 'node:crypto';
import { once } from 'node:events';

const root = fileURLToPath(new URL('../', import.meta.url));

function run(command, args, cwd, options = {}) {
  const result = spawnSync(command, args, { cwd, encoding: 'utf8', timeout: 300000, maxBuffer: 8 * 1024 * 1024, ...options });
  if (result.status !== 0) throw new Error(`${command} failed: ${result.stdout ?? ''}${result.stderr ?? result.error?.message ?? ''}`);
  return result.stdout.trim();
}

// bwrap exposes system libraries and installed prerequisites, never the original
// checkout, developer HOME/cache or sibling projects. The Linux kernel is shared.
async function isolated(temporary, checkout) {
  await mkdir(join(temporary, 'runtime-tmp'));
  const tools = join(temporary, 'tools'); await mkdir(tools); await mkdir(join(tools, 'go')); await mkdir(join(tools, 'node'));
  const goRoot = run('go', ['env', 'GOROOT'], root, { env: { ...process.env, GOTOOLCHAIN: 'local' } });
  await cp(dirname(dirname(process.env.npm_execpath)), join(tools, 'npm'), { recursive: true });
  const node = process.execPath;
  const args = ['--die-with-parent', '--unshare-all', '--share-net', '--clearenv'];
  for (const path of ['/usr', '/bin', '/sbin', '/lib', '/lib64', '/etc']) args.push('--ro-bind-try', path, path);
  args.push('--ro-bind-try', '/run/systemd/resolve', '/run/systemd/resolve');
  if (process.env.WEB_TEST_CHROMIUM) {
    const browser = await realpath(process.env.WEB_TEST_CHROMIUM);
    if (!browser.startsWith('/usr/') && !browser.startsWith('/opt/')) throw new Error('Use an installed system test browser, never the developer browser cache.');
    args.push('--ro-bind', dirname(browser), dirname(browser), '--setenv', 'WEB_TEST_CHROMIUM', browser);
  }
  args.push('--proc', '/proc', '--dev', '/dev', '--bind', join(temporary, 'runtime-tmp'), '/tmp', '--dir', '/home', '--dir', '/work', '--bind', checkout, '/work/checkout', '--ro-bind', tools, '/tools', '--ro-bind', goRoot, '/tools/go', '--ro-bind', dirname(node), '/tools/node');
  for (const [key, value] of Object.entries({ PATH: '/tools/node:/tools/go/bin:/usr/bin:/bin', HOME: '/home/operator', TMPDIR: '/tmp', npm_config_cache: '/tmp/npm-cache', npm_config_userconfig: '/tmp/empty-npmrc', npm_config_globalconfig: '/tmp/empty-global-npmrc', GOTOOLCHAIN: 'local', GOSUMDB: 'sum.golang.org', GOPROXY: 'https://proxy.golang.org', GOMODCACHE: '/tmp/go-mod', GOCACHE: '/tmp/go-build', GOMAXPROCS: '2', PLAYWRIGHT_BROWSERS_PATH: '/tmp/browsers', npm_execpath: '/tools/npm/bin/npm-cli.js' })) args.push('--setenv', key, value);
  args.push('--dir', '/home/operator', '--chdir', '/work/checkout', '/tools/node/node', 'scripts/clone-smoke.mjs', '--inside');
  return run('bwrap', args, root, { timeout: 600000 });
}

async function acceptance() {
  const { runWorkflow, restoreWorkflow, workflowCommands } = await import('./clone-workflow.mjs');
  const work = await mkdtemp(join(tmpdir(), 'attestara-c30-value-'));
  let server; let previousRunId;
  const environment = { ...process.env, npm_execpath: process.env.npm_execpath };
  const stop = async () => {
    if (!server) return;
    const child = server; server = undefined;
    if (child.exitCode !== null || child.signalCode !== null) return;
    const ended = once(child, 'close');
    process.kill(-child.pid, 'SIGTERM');
    const timer = setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }, 5000);
    await ended; clearTimeout(timer);
  };
  const start = async () => {
    const reservation = http.createServer(); reservation.listen(0, '127.0.0.1'); await once(reservation, 'listening');
    const port = reservation.address().port; await new Promise(accept => reservation.close(accept));
    server = spawn(process.execPath, [process.env.npm_execpath, 'start', '--', '--port', String(port), '--store', join(work, 'store')], { cwd: root, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
    await new Promise((accept, reject) => {
      const child = server; let launched = false;
      let output = '';
      const timer = setTimeout(() => reject(new Error('npm start deadline')), 15000);
      const fail = () => { clearTimeout(timer); reject(new Error('npm start exited before readiness')); };
      child.once('error', fail); child.once('exit', fail);
      child.stderr.resume();
      child.stdout.on('data', data => {
        if (launched) return;
        output += data;
        if (output.length > 65536) { clearTimeout(timer); reject(new Error('Startup output exceeds limit')); return; }
        for (const line of output.split('\n')) {
          if (!line.startsWith('{') || !line.endsWith('}')) continue;
          const value = JSON.parse(line);
          if (value.event === 'started') { assert.notEqual(value.runId, previousRunId); previousRunId = value.runId; clearTimeout(timer); launched = true; child.off('exit', fail); accept(); return; }
        }
      });
    });
    const base = `http://127.0.0.1:${port}`;
    const response = await fetch(base + '/readyz');
    assert.equal(response.status, 200);
    assert.equal((await response.json()).storage, 'ready');
    assert.equal((await fetch(base + '/api/v1/unimplemented')).status, 503);
    return port;
  };
  try {
    const setupOutput = run(process.execPath, [process.env.npm_execpath, 'run', 'setup'], root, { env: environment });
    console.log(JSON.stringify({ phase: 'setup', command: 'npm run setup', exit: 0, output: setupOutput }));
    if (!process.env.WEB_TEST_CHROMIUM) run(process.execPath, [process.env.npm_execpath, 'exec', '--no', '--', 'playwright', 'install', 'chromium'], root, { env: environment });
    await start();
    const state = runWorkflow(root, work, join(work, 'store'));
    await stop();
    const port = await start();
    const restored = restoreWorkflow(root, work, state);
    await writeFile(state.passport, restored);
    const browserOutput = run(process.execPath, [process.env.npm_execpath, 'run', 'test:browser', '--', 'tests/browser/clone-to-value.spec.ts'], root, { env: { ...environment, WEB_TEST_PORT: String(port), C30_EXTERNAL_SERVER: '1', C30_ACCEPTANCE_PASSPORT: state.passport } });
    assert.match(browserOutput, /1 passed/);
    console.log(JSON.stringify({ phase: 'browser', command: 'npm run test:browser -- tests/browser/clone-to-value.spec.ts', exit: 0, output: browserOutput }));
    console.log(JSON.stringify({ phase: 'cli', commands: workflowCommands }));
    console.log(JSON.stringify({ passed: true, profile: 'local-cli-snapshots', steps: 10, proof: 'clean isolated git clone -> npm run setup -> npm start -> actual CLI/browser workflow -> service restart -> new-store restore', policy: 'pass', authenticity: 'unverified', host: { platform: platform(), arch: arch(), kernel: release(), cpus: cpus().length, memory: totalmem() }, isolation: 'bubblewrap fresh HOME/npm/Go/browser caches; shared Linux kernel and network; installed prerequisites only', webOnlyProof: { processHealth: true, disabledDomainAdmission: true, certifiesDomain: false }, optionalProvidersExecuted: [], testBrowser: process.env.WEB_TEST_CHROMIUM ? { version: run(process.env.WEB_TEST_CHROMIUM, ['--version'], root), sha256: createHash('sha256').update(await readFile(process.env.WEB_TEST_CHROMIUM)).digest('hex'), source: 'explicit installed system prerequisite; not developer cache' } : { source: 'fresh Playwright download' } }));
  } finally {
    await stop();
    await rm(work, { recursive: true, force: true });
    // Only this namespace's fresh task-owned module cache is removed. Go module
    // directories are read-only, so use Go's cache cleanup before outer removal.
    run('go', ['clean', '-modcache'], root, { env: environment });
  }
}

if (process.argv.includes('--inside')) {
  await acceptance();
} else {
  if (!process.env.npm_execpath) throw new Error('Invoke through pinned npm run test:clone.');
  const temporary = await mkdtemp(join(tmpdir(), 'attestara-c30-clone-'));
  try {
    // Capture the working candidate in a private index; never stage user files.
    const env = { ...process.env, GIT_INDEX_FILE: join(temporary, 'candidate-index'), GIT_AUTHOR_NAME: 'Clone smoke', GIT_AUTHOR_EMAIL: 'clone@example.invalid', GIT_COMMITTER_NAME: 'Clone smoke', GIT_COMMITTER_EMAIL: 'clone@example.invalid' };
    run('git', ['read-tree', 'HEAD'], root, { env });
    run('git', ['add', '-A'], root, { env });
    const tree = run('git', ['write-tree'], root, { env });
    const commit = run('git', ['commit-tree', tree, '-m', 'temporary C30 candidate proof'], root, { env });
    const source = join(temporary, 'source.git');
    run('git', ['init', '--bare', '--quiet', source], temporary);
    await mkdir(join(source, 'objects/info'), { recursive: true });
    await writeFile(join(source, 'objects/info/alternates'), resolve(root, run('git', ['rev-parse', '--git-path', 'objects'], root)) + '\n');
    run('git', ['--git-dir=' + source, 'update-ref', 'refs/heads/snapshot', commit], temporary);
    const checkout = join(temporary, 'checkout');
    run('git', ['clone', '--quiet', '--no-local', '--branch', 'snapshot', source, checkout], temporary);
    assert.equal(run('git', ['rev-parse', 'HEAD^{tree}'], checkout), tree);
    const disk = await statfs(temporary);
    const initialFreeDiskBytes = disk.bavail * disk.bsize;
    assert.ok(initialFreeDiskBytes >= 10240 * 1024 * 1024, 'C30 clean acceptance requires at least 10240 MiB initially free');
    console.log(await isolated(temporary, checkout));
    let cgroup = { available: false };
    try {
      const path = (await readFile('/proc/self/cgroup', 'utf8')).split('\n').find(line => line.startsWith('0::')).slice(3);
      const base = '/sys/fs/cgroup' + path;
      cgroup = { available: true, cpuMax: (await readFile(base + '/cpu.max', 'utf8')).trim(), memoryMax: (await readFile(base + '/memory.max', 'utf8')).trim(), memoryPeak: (await readFile(base + '/memory.peak', 'utf8')).trim() };
    } catch {}
    console.log(JSON.stringify({ candidateTree: tree, transportClone: true, originalIndexUnchanged: true, initialFreeDiskBytes, bwrapVersion: run('bwrap', ['--version'], root), cgroup }));
  } finally { await rm(temporary, { recursive: true, force: true }); }
}
