import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { appendFile, copyFile, mkdtemp, open, readFile, readdir, realpath, rm, stat, statfs, writeFile } from 'node:fs/promises';
import { tmpdir, cpus, totalmem, release, platform, arch } from 'node:os';
import { join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { isDeepStrictEqual } from 'node:util';

const root = fileURLToPath(new URL('../', import.meta.url));
const fixtureRoot = join(root, 'tests/fixtures/attestara');
const defaultScenario = join(root, 'tests/scenarios/target-workload.json');
const maxOutputBytes = 1024 * 1024;

const sha256 = bytes => `sha256:${createHash('sha256').update(bytes).digest('hex')}`;

function requireShape(condition, message) {
  if (!condition) throw new Error(`C28 scenario: ${message}`);
}

export async function loadScenario(path = defaultScenario) {
  const raw = await readFile(path);
  let scenario;
  try { scenario = JSON.parse(raw); } catch { throw new Error('C28 scenario: invalid JSON'); }
  const f = scenario?.fixtures;
  requireShape(scenario?.schema_version === 1 && scenario.license === 'Apache-2.0', 'unsupported version or license');
  requireShape(scenario.target_input_bytes === 100 * 1024 * 1024 && scenario.over_limit_input_bytes === scenario.target_input_bytes + 1, 'input sizes must match the C04 byte limit');
  requireShape(scenario.concurrent_processes === 2 && f && typeof f === 'object', 'invalid process count or fixtures');
  const resolved = {};
  for (const name of ['input', 'policy', 'evidence', 'expected_passport', 'export_manifest', 'export_files']) {
    const value = f[name];
    requireShape(typeof value === 'string' && (name === 'export_files' ? /^C\d\d$/.test(value) : /^C\d\d\/[A-Za-z0-9._/-]+$/.test(value)), `invalid ${name} path`);
    const location = await realpath(join(fixtureRoot, value));
    requireShape(location.startsWith(fixtureRoot + sep), `${name} escapes fixture root`);
    const info = await stat(location);
    requireShape(name === 'export_files' ? info.isDirectory() : info.isFile(), `invalid ${name} type`);
    resolved[name] = location;
  }
  return { scenario, resolved, raw };
}

function runProcess(command, args, { timeoutMs = 120000, signalAfterMs } = {}) {
  return new Promise((resolveResult, reject) => {
    const started = process.hrtime.bigint();
    const child = spawn(command, args, { cwd: root, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
    const stdout = [];
    const stderr = [];
    let outBytes = 0;
    let errBytes = 0;
    let timedOut = false;
    const send = signal => { try { process.kill(-child.pid, signal); } catch { /* process already exited */ } };
    const timeout = setTimeout(() => { timedOut = true; send('SIGKILL'); }, timeoutMs);
    const cancel = signalAfterMs === undefined ? undefined : setTimeout(() => send('SIGINT'), signalAfterMs);
    child.stdout.on('data', chunk => {
      outBytes += chunk.length;
      if (outBytes > maxOutputBytes) send('SIGKILL');
      else stdout.push(chunk);
    });
    child.stderr.on('data', chunk => {
      errBytes += chunk.length;
      if (errBytes > maxOutputBytes) send('SIGKILL');
      else stderr.push(chunk);
    });
    child.on('error', error => { clearTimeout(timeout); if (cancel) clearTimeout(cancel); reject(error); });
    child.on('close', (code, signal) => {
      clearTimeout(timeout);
      if (cancel) clearTimeout(cancel);
      resolveResult({ code, signal, wall_ms: Number(process.hrtime.bigint() - started) / 1e6, stdout: Buffer.concat(stdout).toString(), stderr: Buffer.concat(stderr).toString(), timed_out: timedOut, output_limit: outBytes > maxOutputBytes || errBytes > maxOutputBytes });
    });
  });
}

function assertExit(result, expected, name) {
  if (result.timed_out || result.output_limit || result.code !== expected) throw new Error(`${name}: expected exit ${expected}, got ${result.code ?? result.signal}; ${result.stderr.slice(0, 300)}`);
}

async function measured(binary, args, dir, name) {
  const metrics = join(dir, `${name}.time`);
  const result = await runProcess('/usr/bin/time', ['-f', '%U %S %M', '-o', metrics, binary, ...args]);
  const values = (await readFile(metrics, 'utf8')).trim().split('\n').at(-1).split(/\s+/).map(Number);
  if (values.length !== 3 || values.some(value => !Number.isFinite(value))) throw new Error(`${name}: invalid GNU time metrics`);
  return { ...result, cpu_user_seconds: values[0], cpu_system_seconds: values[1], max_rss_kib: values[2] };
}

async function padJSON(input, output, size) {
  const data = await readFile(input);
  if (data.length > size) throw new Error('C28 scenario: input exceeds target size');
  await writeFile(output, data);
  const file = await open(output, 'a');
  const chunk = Buffer.alloc(1024 * 1024, 0x20);
  let remaining = size - data.length;
  try {
    for (; remaining >= chunk.length; remaining -= chunk.length) await file.writeFile(chunk);
    if (remaining) await file.writeFile(chunk.subarray(0, remaining));
  } finally { await file.close(); }
}

async function diskBytes(dir) {
  let total = 0;
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    total += entry.isDirectory() ? await diskBytes(path) : (await stat(path)).size;
  }
  return total;
}

async function cgroup() {
  const line = (await readFile('/proc/self/cgroup', 'utf8')).split('\n').find(item => item.startsWith('0::'));
  if (!line) return { available: false };
  const base = join('/sys/fs/cgroup', line.slice(3));
  try {
    const [cpu, memory, peak] = await Promise.all(['cpu.max', 'memory.max', 'memory.peak'].map(name => readFile(join(base, name), 'utf8')));
    const [quota, period] = cpu.trim().split(' ');
    return { available: true, cpu_quota: quota === 'max' ? null : Number(quota) / Number(period), memory_limit_bytes: memory.trim() === 'max' ? null : Number(memory.trim()), memory_peak_bytes: Number(peak.trim()) };
  } catch { return { available: false }; }
}

async function sourceIdentity() {
  const listing = await runProcess('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard']);
  assertExit(listing, 0, 'source listing');
  const excluded = new Set(['plans/progress.json', 'plans/requirements.json', 'plans/project-state.json', 'docs/agents/SESSION_HANDOFF.json', 'apps/web/src/generated/project.json', 'SHA256SUMS.txt']);
  const hash = createHash('sha256');
  let count = 0;
  for (const relative of listing.stdout.split('\0').filter(Boolean).sort()) {
    if (excluded.has(relative) || relative.startsWith('docs/evidence/')) continue;
    const bytes = await readFile(join(root, relative));
    hash.update(relative).update('\0').update(createHash('sha256').update(bytes).digest()).update('\n');
    count++;
  }
  return { sha256: `sha256:${hash.digest('hex')}`, files: count };
}

export async function benchmark(path = defaultScenario) {
  const { scenario, resolved, raw } = await loadScenario(path);
  const c28Files = ['README.md', 'positive.json', 'negative.json', 'expected.json'];
  const c28Hashes = Object.fromEntries(await Promise.all(c28Files.map(async name => [name, sha256(await readFile(join(fixtureRoot, 'C28', name)))])));
  const assertions = JSON.parse(await readFile(join(fixtureRoot, 'C28/expected.json'), 'utf8'));
  if (assertions.schema_version !== 1 || assertions.license !== 'Apache-2.0' || assertions.passport_fixture !== scenario.fixtures.expected_passport || assertions.over_limit_exit_code !== 2) throw new Error('C28 expected fixture is invalid');
  const versions = await Promise.all([runProcess('go', ['version']), runProcess('npm', ['--version']), runProcess('node', ['--version'])]);
  for (const [index, expected] of ['go1.27.2', '11.9.0', 'v24.19.0'].entries()) {
    assertExit(versions[index], 0, 'tool version');
    if (!versions[index].stdout.includes(expected)) throw new Error(`required tool version unavailable: ${expected}`);
  }
  await stat('/usr/bin/time');
  const source = await sourceIdentity();
  const initialDisk = await statfs(tmpdir(), { bigint: true });
  const temporary = await mkdtemp(join(tmpdir(), 'attestara-c28-'));
  try {
    const binary = join(temporary, 'attestara');
    const build = await runProcess('go', ['build', '-o', binary, './cmd/attestara']);
    assertExit(build, 0, 'build CLI');
    const input = join(temporary, 'target-input.json');
    const overLimit = join(temporary, 'over-limit-input.json');
    await padJSON(resolved.input, input, scenario.target_input_bytes);
    await copyFile(input, overLimit);
    await appendFile(overLimit, ' ');
    const expected = JSON.parse(await readFile(resolved.expected_passport, 'utf8'));
    const evaluateArgs = ['evaluate', '--input', input, '--policy', resolved.policy, '--evidence', resolved.evidence, '--freshness', 'fresh'];
    const runs = [];
    let peakDisk = await diskBytes(temporary);
    for (let repeat = 0; repeat < 2; repeat++) {
      const evaluation = await measured(binary, evaluateArgs, temporary, `evaluate-${repeat}`);
      assertExit(evaluation, 0, 'evaluate');
      const passport = JSON.parse(evaluation.stdout);
      if (!isDeepStrictEqual(passport, expected) || passport.verification !== assertions.verification || passport.policy_decision !== assertions.policy_decision) throw new Error('evaluate: typed passport changed or trust was promoted');
      const passportPath = join(temporary, `passport-${repeat}.json`);
      await writeFile(passportPath, evaluation.stdout);
      const verification = await measured(binary, ['verify', '--passport', passportPath], temporary, `verify-${repeat}`);
      assertExit(verification, 0, 'verify');
      if (!isDeepStrictEqual(JSON.parse(verification.stdout), expected) || !verification.stderr.includes('signer and bundle trust were not assessed')) throw new Error('verify: result or trust boundary changed');
      const archive = join(temporary, `export-${repeat}.zip`);
      const exported = await measured(binary, ['export', '--input', resolved.export_manifest, '--files', resolved.export_files, '--output', archive], temporary, `export-${repeat}`);
      assertExit(exported, 0, 'export');
      const archiveBytes = await readFile(archive);
      if (!archiveBytes.length) throw new Error('export: empty ZIP');
      peakDisk = Math.max(peakDisk, await diskBytes(temporary));
      runs.push({ evaluation: metricsOnly(evaluation), verification: metricsOnly(verification), export: { ...metricsOnly(exported), zip_bytes: archiveBytes.length, zip_sha256: sha256(archiveBytes) }, passport_sha256: sha256(Buffer.from(evaluation.stdout)) });
    }
    if (assertions.repeat_passport_equal && (runs[0].passport_sha256 !== runs[1].passport_sha256 || runs[0].export.zip_sha256 !== runs[1].export.zip_sha256)) throw new Error('restart repeat changed passport or ZIP bytes');
    const overload = await measured(binary, ['evaluate', '--input', overLimit, '--policy', resolved.policy, '--evidence', resolved.evidence, '--freshness', 'fresh'], temporary, 'over-limit');
    assertExit(overload, assertions.over_limit_exit_code, 'over-limit');
    if (overload.stdout !== '') throw new Error('over-limit: produced a passport');
    const cancel = await runProcess(binary, evaluateArgs, { signalAfterMs: 10 });
    if (cancel.code === 0 || cancel.stdout.includes('"policy_decision":"pass"')) throw new Error('cancel: evaluation completed or published pass');
    const saturationStarted = process.hrtime.bigint();
    const concurrent = await Promise.all(Array.from({ length: scenario.concurrent_processes }, (_, index) => measured(binary, evaluateArgs, temporary, `concurrent-${index}`)));
    for (const result of concurrent) {
      assertExit(result, 0, 'concurrent evaluate');
      if (!isDeepStrictEqual(JSON.parse(result.stdout), expected)) throw new Error('concurrent evaluate: typed result changed');
    }
    peakDisk = Math.max(peakDisk, await diskBytes(temporary));
    const disk = await statfs(temporary, { bigint: true });
    const group = await cgroup();
    const diskTotal = Number(initialDisk.blocks * initialDisk.bsize);
    const diskInitialFree = Number(initialDisk.bavail * initialDisk.bsize);
    const cpuMemoryProfile = group.cpu_quota === 2 && group.memory_limit_bytes === 2048 * 1024 * 1024;
    const diskProfile = diskInitialFree >= 10240 * 1024 * 1024 && diskTotal <= 10241 * 1024 * 1024;
    const fixtureBytes = {};
    for (const [name, file] of Object.entries(resolved)) {
      if (name === 'export_files') continue;
      fixtureBytes[name] = { sha256: sha256(await readFile(file)), path: scenario.fixtures[name] };
    }
    fixtureBytes.export_files = { path: scenario.fixtures.export_files, files: {} };
    for (const entry of (await readdir(resolved.export_files)).sort()) {
      const file = join(resolved.export_files, entry);
      if ((await stat(file)).isFile()) fixtureBytes.export_files.files[entry] = sha256(await readFile(file));
    }
    return {
      schema_version: 1, task: 'C28', source_identity: source.sha256, source_files: source.files, scenario_sha256: sha256(raw),
      fixtures: { ...fixtureBytes, C28: c28Hashes },
      toolchain: { go: versions[0].stdout.trim(), npm: versions[1].stdout.trim(), node: versions[2].stdout.trim() },
      environment: { platform: platform(), arch: arch(), kernel: release(), logical_cpus: cpus().length, host_memory_bytes: totalmem(), cgroup: group, disk_initial_free_bytes: diskInitialFree, free_disk_bytes: Number(disk.bavail * disk.bsize), disk_limit_bytes: diskTotal },
      cpu_memory_profile_verified: cpuMemoryProfile,
      disk_profile_verified: diskProfile,
      target_profile_verified: cpuMemoryProfile && diskProfile,
      target_profile_note: diskProfile ? 'CLI workload measured on a filesystem with at least 10 GiB initially free and at most 10 GiB plus 1 MiB total; this is not a complete engine or provider measurement.' : 'Disk availability was observed without an isolated 10 GiB free-disk profile.',
      target: { input_bytes: scenario.target_input_bytes, runs, peak_temporary_file_bytes: peakDisk },
      over_limit: { input_bytes: scenario.over_limit_input_bytes, ...metricsOnly(overload), stdout_empty: true },
      cancellation: { exit_code: cancel.code, signal: cancel.signal, wall_ms: cancel.wall_ms, stdout_empty: cancel.stdout === '' },
      saturation: { concurrent_processes: scenario.concurrent_processes, wall_ms: Number(process.hrtime.bigint() - saturationStarted) / 1e6, runs: concurrent.map(metricsOnly) },
      providers: { required: [], executed: [] },
    };
  } finally { await rm(temporary, { recursive: true, force: true }); }
}

function metricsOnly(result) {
  return { exit_code: result.code, wall_ms: result.wall_ms, cpu_user_seconds: result.cpu_user_seconds, cpu_system_seconds: result.cpu_system_seconds, max_rss_kib: result.max_rss_kib };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  benchmark(process.argv[2] ? resolve(process.argv[2]) : defaultScenario).then(result => console.log(JSON.stringify(result, null, 2))).catch(error => {
    console.error(`C28 benchmark failed: ${error.message}`);
    process.exitCode = 1;
  });
}
