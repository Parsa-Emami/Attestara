import { spawn, execFileSync } from 'node:child_process';
import { openSync, closeSync, statSync } from 'node:fs';
import { readFile, writeFile, mkdir, mkdtemp, lstat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { join, resolve, relative, isAbsolute } from 'node:path';
import { cpus, totalmem, release } from 'node:os';
import { fileURLToPath } from 'node:url';
import { suites } from './verify-engine-tests.mjs';
import { verifyInventory } from './package-release.mjs';

const sha256 = bytes => 'sha256:' + createHash('sha256').update(bytes).digest('hex');
const json = async path => JSON.parse(await readFile(path, 'utf8'));
export const requiredGatePhases = ["source-before", "python-version", "git-version", "contracts", "engineering-tests", "setup", "resources", "engine-discovery", "go-build", "go-vet", "go-tests", "web-check", "typecheck", "nginx-version", "guide-and-nginx", "browser-version", "browser-tests", "clone-workflow", "package-workflow", "web-package", "dependency-audit", "security-reporting", "javascript-syntax", "diff-check", "source-after"];
const check = (condition, message) => { if (!condition) throw new Error(message); };

export function validateGoRun(output, required = suites) {
  const packages = new Map(), tests = new Map();
  const requiredPackages = new Set(Object.keys(required).map(pkg => 'attestara/' + pkg.slice(2)));
  const noTests = new Set();
  let passed = 0;
  for (const line of output.split('\n').filter(line => line.trim())) {
    const event = JSON.parse(line);
    check(event.Action !== 'fail' && !(event.Action === 'skip' && event.Test), 'Go failure/skip cannot certify the release');
    if (!event.Test) {
      if (event.Action === 'output' && event.Output?.includes('[no test files]')) noTests.add(event.Package);
      if (event.Action === 'start') packages.set(event.Package, 'started');
      if (event.Action === 'skip') {
        check(packages.get(event.Package) === 'started' && noTests.has(event.Package) && !requiredPackages.has(event.Package),
          'Required Go package skipped');
        packages.set(event.Package, 'no-tests');
      }
      if (event.Action === 'pass') {
        check(packages.get(event.Package) === 'started', 'Go package completion without start');
        packages.set(event.Package, 'passed');
      }
    } else {
      check(packages.get(event.Package) === 'started', 'Go test event outside a running package');
      const key = event.Package + '/' + event.Test;
      if (event.Action === 'run') tests.set(key, 'running');
      if (event.Action === 'pass') {
        check(tests.get(key) === 'running', 'Go test pass without run');
        tests.set(key, 'passed'); passed++;
      }
    }
  }
  for (const [pkg, names] of Object.entries(required)) {
    const path = 'attestara/' + pkg.slice(2);
    check(packages.get(path) === 'passed', 'Required Go package did not finish: ' + path);
    for (const name of names) check(tests.get(path + '/' + name) === 'passed', 'Required Go test did not execute: ' + name);
  }
  check(packages.size > 0 && [...packages.values()].every(state => ['passed', 'no-tests'].includes(state)), 'Incomplete Go package stream');
  check([...tests.values()].every(state => state === 'passed'), 'Incomplete Go test stream');
  return { packages: [...packages.values()].filter(state => state === 'passed').length, passed, skipped: 0 };
}

export async function runCommand(command, args, { cwd, log, env = process.env, timeoutMs = 600000, signal }) {
  const started = performance.now();
  const fd = openSync(log, 'wx', 0o600);
  const result = { command: [command, ...args], exit_code: null, signal: null, timed_out: false, interrupted: false };
  try {
    await new Promise(accept => {
      const child = spawn(command, args, { cwd, env, detached: true, stdio: ['ignore', fd, fd] });
      let killTimer;
      const stop = () => {
        try { process.kill(-child.pid, 'SIGTERM'); } catch {}
        killTimer ??= setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }, 5000);
      };
      const interrupted = () => { result.interrupted = true; stop(); };
      signal?.addEventListener('abort', interrupted, { once: true });
      const timer = setTimeout(() => { result.timed_out = true; stop(); }, timeoutMs);
      const limit = setInterval(() => {
        if (statSync(log).size > 64 * 1024 * 1024) { result.operational_error = 'output_limit'; stop(); }
      }, 100);
      child.once('error', error => { result.operational_error = error.code ?? 'spawn_error'; });
      child.once('close', (code, childSignal) => {
        clearTimeout(timer); clearTimeout(killTimer); clearInterval(limit);
        signal?.removeEventListener('abort', interrupted);
        result.exit_code = code; result.signal = childSignal; accept();
      });
      if (signal?.aborted) interrupted();
    });
  } finally { closeSync(fd); }
  result.elapsed_ms = Math.round(performance.now() - started);
  result.output_sha256 = sha256(await readFile(log));
  return result;
}

function lastObject(output) {
  for (let position = output.lastIndexOf('\n{'); position >= 0; position = output.lastIndexOf('\n{', position - 1)) {
    try { return JSON.parse(output.slice(position + 1)); } catch {}
    if (position === 0) break;
  }
  throw new Error('Missing structured integration result');
}

export async function runReleaseGate(root) {
  root = resolve(root);
  await mkdir(join(root, 'dist/release-gate'), { recursive: true });
  const directory = await mkdtemp(join(root, 'dist/release-gate/run-'));
  const controller = new AbortController();
  const abort = () => controller.abort();
  process.once('SIGINT', abort); process.once('SIGTERM', abort);
  const record = {
    schema_version: 1, scope: 'release-gate', passed: false,
    profile: 'local-cli-snapshots', started_at: new Date().toISOString(), source: null,
    run: { id: process.env.GITHUB_RUN_ID ?? null, attempt: process.env.GITHUB_RUN_ATTEMPT ?? null,
      kind: process.env.GITHUB_ACTIONS === 'true' ? 'github-actions' : 'local' },
    hardware: { platform: process.platform, arch: process.arch, kernel: release(), logical_cpus: cpus().length,
      host_memory_bytes: totalmem(), capacity_certified: false },
    tools: {}, commands: [], results: {}, blockers: [], published: false,
  };
  const environment = { ...process.env, GOTOOLCHAIN: 'local' };
  const npm = process.env.npm_execpath;
  let exitCode = 1;
  const phase = async (id, command, args) => {
    check(!controller.signal.aborted, 'Gate interrupted');
    console.log(JSON.stringify({ phase: id, status: 'running' }));
    const log = join(directory, id + '.log');
    const result = await runCommand(command, args, { cwd: root, log, env: environment, signal: controller.signal });
    record.commands.push({ id, ...result, output_path: relative(root, log) });
    if (result.operational_error || result.timed_out || result.interrupted) {
      exitCode = 1; throw new Error('Operational failure: ' + id);
    }
    if (result.exit_code !== 0) {
      exitCode = 2; throw new Error('Readiness check failed: ' + id);
    }
    console.log(JSON.stringify({ phase: id, status: 'passed' }));
    return readFile(log, 'utf8');
  };
  const npmRun = (id, ...args) => phase(id, process.execPath, [npm, ...args]);
  try {
    const lock = await json(join(root, 'docs/engineering/toolchain-lock.json'));
    check(process.platform === 'linux' && process.arch === 'x64' && process.version === 'v' + lock.runtime.node,
      'Locked Linux x64 Node required');
    check(npm && execFileSync(process.execPath, [npm, '--version'], { encoding: 'utf8', timeout: 120000 }).trim() === lock.runtime.npm,
      'Invoke through pinned npm ' + lock.runtime.npm);
    const go = execFileSync('go', ['env', 'GOVERSION', 'GOROOT'], { env: environment, encoding: 'utf8', timeout: 120000 }).trim().split('\n');
    check(go[0] === 'go' + lock.resources.find(item => item.id === 'go').version, 'Locked Go on PATH required');
    record.tools = { node: { version: process.version, sha256: sha256(await readFile(process.execPath)) },
      npm: { version: lock.runtime.npm, cli_sha256: sha256(await readFile(npm)) },
      go: { version: go[0], sha256: sha256(await readFile(join(go[1], 'bin/go'))) } };
    record.source = JSON.parse(await phase('source-before', 'python3', ['scripts/verify_ci_source.py', '--snapshot']));
    const resource = lock.resources.find(item => item.id === 'opa');
    const opa = environment.ATTESTARA_TEST_OPA ?? join(directory, 'opa');
    if (!environment.ATTESTARA_TEST_OPA) {
      const response = await fetch(resource.artifact.url, { signal: AbortSignal.timeout(120000) });
      check(response.ok, 'Pinned OPA download unavailable');
      const bytes = Buffer.from(await response.arrayBuffer());
      check(sha256(bytes) === 'sha256:' + resource.artifact.sha256, 'Pinned OPA checksum mismatch');
      await writeFile(opa, bytes, { flag: 'wx', mode: 0o700 });
    }
    check(isAbsolute(opa) && (await lstat(opa)).isFile() && sha256(await readFile(opa)) === 'sha256:' + resource.artifact.sha256,
      'Pinned regular OPA executable required');
    environment.ATTESTARA_TEST_OPA = opa;
    record.tools.opa = { version: resource.version, sha256: 'sha256:' + resource.artifact.sha256 };
    record.tools.python = { version: (await phase('python-version', 'python3', ['--version'])).trim() };
    record.tools.git = { version: (await phase('git-version', 'git', ['--version'])).trim() };
    await phase('contracts', 'python3', ['scripts/validate_repo.py']);
    await phase('engineering-tests', 'python3', ['-m', 'unittest', 'discover', '-s', 'tests/engineering', '-p', 'test_*.py', '-v']);
    await npmRun('setup', 'run', 'setup');
    await phase('resources', process.execPath, ['scripts/verify-resources.mjs']);
    await phase('engine-discovery', process.execPath, ['scripts/verify-engine-tests.mjs']);
    await phase('go-build', 'go', ['build', './...']);
    await phase('go-vet', 'go', ['vet', './...']);
    const goOutput = await phase('go-tests', 'go', ['test', '-json', '-count=1', './...']);
    exitCode = 2;
    record.results.go = validateGoRun(goOutput);
    await npmRun('web-check', 'run', 'check');
    await npmRun('typecheck', 'run', 'typecheck');
    record.tools.nginx = { version: (await phase('nginx-version', 'nginx', ['-v'])).trim() };
    await phase('guide-and-nginx', process.execPath, ['--test', 'tests/ops/user-guide.test.mjs', 'tests/ops/nginx-local.test.mjs']);
    const { chromium } = await import('@playwright/test');
    const browser = process.env.WEB_TEST_CHROMIUM ?? chromium.executablePath();
    record.tools.browser = { version: (await phase('browser-version', browser, ['--version'])).trim(),
      sha256: sha256(await readFile(browser)) };
    await npmRun('browser-tests', 'run', 'test:browser');
    await npmRun('clone-workflow', 'run', 'test:clone');
    const clone = await json(join(root, 'dist/onboarding/C36-run.json'));
    check(clone.passed === true && clone.source.identity === record.source.identity, 'Clone result belongs to a different source');
    record.results.clone = { ...clone.result, source: clone.source, output_sha256: sha256(await readFile(join(root, 'dist/onboarding/C36-run.json'))) };
    const packaged = lastObject(await npmRun('package-workflow', 'run', 'test:package'));
    check(packaged.passed === true && packaged.candidateTree === record.source.tree, 'Package result belongs to a different source tree');
    const artifacts = resolve(packaged.artifacts);
    check(artifacts.startsWith(join(root, 'dist/packages/')) && /^[a-f0-9]{40}$/.test(packaged.candidateCommit), 'Unsafe package output');
    await verifyInventory(artifacts, packaged.files);
    record.results.package = { candidate_commit: packaged.candidateCommit, candidate_tree: packaged.candidateTree,
      files: packaged.files, checks: packaged.checks, path: relative(root, artifacts) };
    await npmRun('web-package', 'run', 'package:web');
    record.results.audit = JSON.parse(await npmRun('dependency-audit', 'audit', '--audit-level=high', '--json'));
    record.results.reporting = JSON.parse(await npmRun('security-reporting', 'run', '--silent', 'security:check'));
    await phase('javascript-syntax', process.execPath, ['--check', 'scripts/release-gate.mjs']);
    await phase('diff-check', 'git', ['diff', '--check']);
    const final = JSON.parse(await phase('source-after', 'python3', ['scripts/verify_ci_source.py', '--snapshot']));
    check(final.identity === record.source.identity && final.commit === record.source.commit, 'Source changed during release verification');
    check(!controller.signal.aborted, 'Gate interrupted');
    record.fixtures = Object.fromEntries(Object.entries(record.source.files).filter(([path]) => path.startsWith('tests/fixtures/attestara/')));
    record.results.source_unchanged = true;
    record.results.lint = 'No standalone JavaScript lint command; actual Go vet, syntax and diff checks are recorded.';
    record.passed = true; exitCode = 0;
  } catch (error) {
    record.blockers.push(error.message);
    console.error(error.message);
  } finally {
    process.removeListener('SIGINT', abort); process.removeListener('SIGTERM', abort);
    record.finished_at = new Date().toISOString();
    await writeFile(join(directory, 'report.json'), JSON.stringify(record, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
    console.log(JSON.stringify({ passed: record.passed, report: relative(root, join(directory, 'report.json')), exit_code: exitCode }));
  }
  return exitCode;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = await runReleaseGate(fileURLToPath(new URL('../', import.meta.url)));
}
