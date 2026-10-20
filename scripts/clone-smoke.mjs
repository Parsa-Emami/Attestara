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
const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');

function run(command, args, cwd, options = {}) {
  const result = spawnSync(command, args, { cwd, encoding: 'utf8', timeout: 300000, maxBuffer: 8 * 1024 * 1024, ...options });
  if (result.status !== 0) throw new Error(`${command} failed: ${result.stdout ?? ''}${result.stderr ?? result.error?.message ?? ''}`);
  return result.stdout.trim();
}

// bwrap exposes system libraries and installed prerequisites, never the original
// checkout, developer HOME/cache or sibling projects. The Linux kernel is shared.
async function isolated(temporary, checkout, sourceCommit, candidateCommit, tree) {
  if (process.env.GITHUB_ACTIONS === 'true') {
    const runtime = join(temporary, 'runtime');
    const environment = {
      ...process.env,
      HOME: join(runtime, 'home'),
      TMPDIR: join(runtime, 'tmp'),
      npm_config_cache: join(runtime, 'npm-cache'),
      npm_config_userconfig: join(runtime, 'empty-npmrc'),
      npm_config_globalconfig: join(runtime, 'empty-global-npmrc'),
      GOMODCACHE: join(runtime, 'go-mod'),
      GOCACHE: join(runtime, 'go-build'),
      PLAYWRIGHT_BROWSERS_PATH: join(runtime, 'browsers'),
      C36_SOURCE_COMMIT: sourceCommit,
      C36_CANDIDATE_COMMIT: candidateCommit,
      C36_CANDIDATE_TREE: tree,
    };
    for (const path of [environment.HOME, environment.TMPDIR, environment.npm_config_cache, environment.GOMODCACHE, environment.GOCACHE, environment.PLAYWRIGHT_BROWSERS_PATH]) await mkdir(path, { recursive: true });
    return run(process.execPath, ['scripts/clone-smoke.mjs', '--inside'], checkout, { env: environment, timeout: 600000 });
  }
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
  for (const [key, value] of Object.entries({ PATH: '/tools/node:/tools/go/bin:/usr/bin:/bin', HOME: '/home/operator', TMPDIR: '/tmp', npm_config_cache: '/tmp/npm-cache', npm_config_userconfig: '/tmp/empty-npmrc', npm_config_globalconfig: '/tmp/empty-global-npmrc', GOTOOLCHAIN: 'local', GOSUMDB: 'sum.golang.org', GOPROXY: 'https://proxy.golang.org', GOMODCACHE: '/tmp/go-mod', GOCACHE: '/tmp/go-build', GOMAXPROCS: '2', PLAYWRIGHT_BROWSERS_PATH: '/tmp/browsers', npm_execpath: '/tools/npm/bin/npm-cli.js', C36_SOURCE_COMMIT: sourceCommit, C36_CANDIDATE_COMMIT: candidateCommit, C36_CANDIDATE_TREE: tree })) args.push('--setenv', key, value);
  args.push('--dir', '/home/operator', '--chdir', '/work/checkout', '/tools/node/node', 'scripts/clone-smoke.mjs', '--inside');
  return run('bwrap', args, root, { timeout: 600000 });
}

async function acceptance() {
  const { runWorkflow, restoreWorkflow, validateOnboardingScenario, validateOnboardingEnvironment, workflowCommands } = await import('./clone-workflow.mjs');
  const work = await mkdtemp(join(tmpdir(), 'attestara-c30-value-'));
  let server; let previousRunId;
  const environment = { ...process.env, npm_execpath: process.env.npm_execpath };
  const steps = [];
  const commandOffset = workflowCommands.length;
  const reportPath = join(root, 'dist/onboarding/C36-run.json');
  const record = {
    schema_version: 1, task: 'C36', passed: false,
    source: { commit: process.env.C36_SOURCE_COMMIT ?? null, candidate_commit: process.env.C36_CANDIDATE_COMMIT ?? null, tree: process.env.C36_CANDIDATE_TREE ?? null, identity: null },
    environment: null, fixtures: null, steps, commands: [], failures: [], limitations: [],
  };
  const step = async (id, action) => {
    const started = performance.now();
    try {
      const result = await action();
      steps.push({ id, outcome: 'passed', elapsed_ms: Math.round(performance.now() - started) });
      return result;
    } catch (error) {
      steps.push({ id, outcome: 'failed', elapsed_ms: Math.round(performance.now() - started) });
      record.failures.push({ step: id, error_type: error?.name ?? 'Error' });
      throw error;
    }
  };
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
      const child = server; let launched = false; let output = '';
      const timer = setTimeout(() => reject(new Error('npm start deadline')), 15000);
      const fail = () => { clearTimeout(timer); reject(new Error('npm start exited before readiness')); };
      child.once('error', fail); child.once('exit', fail); child.stderr.resume();
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
    const scenario = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
    const onboarding = validateOnboardingScenario(scenario);
    const actualToolchain = await step('prerequisites', async () => {
      const lock = JSON.parse(await readFile(join(root, 'docs/engineering/toolchain-lock.json'), 'utf8'));
      assert.equal(lock.runtime.node, onboarding.toolchain.node, 'C36 Node pin differs from the resource lock');
      assert.equal(lock.resources.find(resource => resource.id === 'npm')?.version, onboarding.toolchain.npm, 'C36 npm pin differs from the resource lock');
      assert.equal(lock.resources.find(resource => resource.id === 'go')?.version, onboarding.toolchain.go, 'C36 Go pin differs from the resource lock');
      assert.ok(process.env.npm_execpath, 'C36 prerequisite unavailable: npm');
      const npm = run(process.execPath, [process.env.npm_execpath, '--version'], root, { env: environment });
      const go = run('go', ['version'], root, { env: { ...environment, GOTOOLCHAIN: 'local' } }).match(/go(\d+\.\d+\.\d+)/)?.[1];
      const versions = { node: process.version.slice(1), npm, go, platform: `${platform()}-${arch()}` };
      validateOnboardingEnvironment(onboarding, versions);
      return versions;
    });
    const source = await sourceIdentity(root);
    const fixtureHashes = await onboardingFixtureHashes(root);
    record.source.identity = source.identity;
    record.source.file_count = source.file_count;
    record.environment = {
      kind: process.env.GITHUB_ACTIONS === 'true' ? 'github-actions-hosted-runner' : 'local-bubblewrap',
      runner_os: process.env.RUNNER_OS ?? platform(), runner_image: process.env.ImageOS ?? null,
      run_id: process.env.GITHUB_RUN_ID ?? null, run_attempt: process.env.GITHUB_RUN_ATTEMPT ?? null,
      runner_source_commit: process.env.GITHUB_SHA ?? process.env.C36_SOURCE_COMMIT ?? null,
      platform: actualToolchain.platform, kernel: release(), logical_cpus: cpus().length, host_memory_bytes: totalmem(),
      toolchain: { ...actualToolchain, git: run('git', ['--version'], root) },
    };
    record.fixtures = fixtureHashes;
    record.limitations = process.env.GITHUB_ACTIONS === 'true' ? [] : ['Local bubblewrap run shares the host kernel and is not independent-runner C36 evidence.'];

    await step('setup', () => run(process.execPath, [process.env.npm_execpath, 'run', 'setup'], root, { env: environment }));
    await step('guide-cli-restart-restore', () => run(process.execPath, ['--test', onboarding.guide_test], root, { env: environment, timeout: 600000 }));
    if (!process.env.WEB_TEST_CHROMIUM) await step('browser-install', () => run(process.execPath, [process.env.npm_execpath, 'exec', '--no', '--', 'playwright', 'install', 'chromium'], root, { env: environment, timeout: 600000 }));
    else steps.push({ id: 'browser-install', outcome: 'not-required', elapsed_ms: 0 });
    const initialPort = await step('service-start', start);
    const state = await step('cli-workflow', () => runWorkflow(root, work, join(work, 'store')));
    await step('service-stop', stop);
    const port = await step('service-restart', start);
    const restored = await step('new-store-restore', async () => {
      const passport = restoreWorkflow(root, work, state);
      await writeFile(state.passport, passport);
      return passport;
    });
    const browserOutput = await step('browser-viewer', () => run(process.execPath, [process.env.npm_execpath, 'run', 'test:browser', '--', onboarding.browser_test], root, { env: { ...environment, WEB_TEST_PORT: String(port), C30_EXTERNAL_SERVER: '1', C30_ACCEPTANCE_PASSPORT: state.passport }, timeout: 600000 }));
    assert.match(browserOutput, /1 passed/);
    assert.equal(JSON.parse(restored).policy_decision, 'pass');
    record.commands = workflowCommands.slice(commandOffset).map(({ command, expected_exit, exit, duration_ms }) => ({ action: command[1], expected_exit, exit, duration_ms }));
    record.passed = true;
    record.result = { profile: 'local-cli-snapshots', scenario_steps: scenario.steps.length, policy_decision: 'pass', authenticity: 'unverified', restart_preserved: true, new_store_restore: true, browser_viewer: 'passed', user_source_execution: false, optional_providers_executed: [] };
    console.log(JSON.stringify({ phase: 'C36 onboarding', passed: true, source_identity: source.identity, environment: record.environment, fixtures: fixtureHashes, steps, commands: record.commands }, null, 2));
    console.log(JSON.stringify({ passed: true, profile: 'local-cli-snapshots', proof: 'fresh clone -> public guide CLI/restart/restore -> supervised CLI/viewer workflow -> restart -> new-store restore', isolation: record.environment.kind, browser: process.env.WEB_TEST_CHROMIUM ? { version: run(process.env.WEB_TEST_CHROMIUM, ['--version'], root), sha256: sha256(await readFile(process.env.WEB_TEST_CHROMIUM)), source: 'explicit installed system prerequisite' } : { source: 'fresh Playwright download' }, service_start_port: initialPort, user_source_execution: false, optional_providers_executed: [] }, null, 2));
  } finally {
    await stop();
    await rm(work, { recursive: true, force: true });
    run('go', ['clean', '-modcache'], root, { env: environment });
    await mkdir(join(root, 'dist/onboarding'), { recursive: true });
    record.elapsed_ms = steps.reduce((total, item) => total + item.elapsed_ms, 0);
    await writeFile(reportPath, JSON.stringify(record, null, 2) + '\n', { mode: 0o600 });
  }
}

async function sourceIdentity(directory) {
  const paths = run('git', ['ls-files', '-z'], directory).split('\0').filter(Boolean).sort();
  const excluded = new Set(['plans/progress.json', 'plans/requirements.json', 'plans/project-state.json', 'docs/agents/SESSION_HANDOFF.json', 'apps/web/src/generated/project.json', 'SHA256SUMS.txt']);
  const digest = createHash('sha256');
  let fileCount = 0;
  for (const path of paths) {
    if (excluded.has(path) || path.startsWith('docs/evidence/')) continue;
    digest.update(path).update('\0').update(createHash('sha256').update(await readFile(join(directory, path))).digest()).update('\n');
    fileCount++;
  }
  return { identity: 'sha256:' + digest.digest('hex'), file_count: fileCount };
}

async function onboardingFixtureHashes(directory) {
  const provenance = JSON.parse(await readFile(join(directory, 'tests/fixtures/attestara/C36/provenance.json'), 'utf8'));
  assert.equal(provenance.license, 'Apache-2.0');
  const files = {};
  for (const [path, expected] of Object.entries(provenance.files)) {
    const actual = 'sha256:' + sha256(await readFile(join(directory, 'tests/fixtures/attestara/C36', path)));
    assert.equal(actual, expected, `C36 fixture checksum mismatch: ${path}`);
    files[path] = actual;
  }
  for (const path of ['tests/scenarios/clone-to-value.json', 'docs/release/USER_GUIDE.md', 'docs/engineering/CLONE_TO_RUN.md']) {
    files[path] = 'sha256:' + sha256(await readFile(join(directory, path)));
  }
  return { license: provenance.license, files };
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
    const sourceCommit = run('git', ['rev-parse', 'HEAD'], root);
    let isolatedOutput;
    let isolatedFailure;
    try { isolatedOutput = await isolated(temporary, checkout, sourceCommit, commit, tree); }
    catch (error) { isolatedFailure = error; }
    let cgroup = { available: false };
    try {
      const path = (await readFile('/proc/self/cgroup', 'utf8')).split('\n').find(line => line.startsWith('0::')).slice(3);
      const base = '/sys/fs/cgroup' + path;
      cgroup = { available: true, cpuMax: (await readFile(base + '/cpu.max', 'utf8')).trim(), memoryMax: (await readFile(base + '/memory.max', 'utf8')).trim(), memoryPeak: (await readFile(base + '/memory.peak', 'utf8')).trim() };
    } catch {}
    const innerPath = join(checkout, 'dist/onboarding/C36-run.json');
    let inner;
    try { inner = JSON.parse(await readFile(innerPath, 'utf8')); } catch {}
    const result = {
      ...(inner ?? { schema_version: 1, task: 'C36', passed: false, steps: [], commands: [], failures: [] }),
      passed: !isolatedFailure && inner?.passed === true,
      source: { ...(inner?.source ?? {}), commit: sourceCommit, candidate_commit: commit, tree },
      transport_clone: true,
      original_index_unchanged: true,
      initial_free_disk_bytes: initialFreeDiskBytes,
      isolation_runtime: process.env.GITHUB_ACTIONS === 'true' ? 'GitHub Actions ephemeral VM' : run('bwrap', ['--version'], root),
      cgroup,
      failures: [...(inner?.failures ?? []), ...(isolatedFailure ? [{ step: 'isolated-runner', error_type: isolatedFailure.name ?? 'Error' }] : [])],
    };
    const output = join(root, 'dist/onboarding/C36-run.json');
    await mkdir(join(root, 'dist/onboarding'), { recursive: true });
    await writeFile(output, JSON.stringify(result, null, 2) + '\n', { mode: 0o600 });
    console.log(JSON.stringify(result, null, 2));
    if (isolatedOutput) console.log(isolatedOutput);
    if (isolatedFailure) throw new Error('C36 onboarding failed inside the clean clone; inspect the structured run artifact and command log.');
  } finally { await rm(temporary, { recursive: true, force: true }); }
}
