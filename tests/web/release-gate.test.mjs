import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, mkdir, writeFile, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { validateGoRun, runCommand, runReleaseGate } from '../../scripts/release-gate.mjs';

const required = { './tests': ['TestBoundary'] };
const events = [
  { Action: 'start', Package: 'attestara/tests' },
  { Action: 'run', Package: 'attestara/tests', Test: 'TestBoundary' },
  { Action: 'pass', Package: 'attestara/tests', Test: 'TestBoundary' },
  { Action: 'pass', Package: 'attestara/tests' },
];
const stream = events => events.map(event => JSON.stringify(event)).join('\n');

test('C39 requires actual package starts/completions and ordered test run/pass events', () => {
  assert.deepEqual(validateGoRun(stream(events), required), { packages: 1, passed: 1, skipped: 0 });
  for (let index = 0; index < events.length; index++) {
    assert.throws(() => validateGoRun(stream(events.filter((_, position) => index !== position)), required));
  }
  assert.throws(() => validateGoRun(stream([events[0], events[2], events[1], events[3]]), required));
  for (const Action of ['skip', 'fail']) assert.throws(() => validateGoRun(stream([...events, { Action }]), required));
  assert.throws(() => validateGoRun('not JSON', required));
  assert.throws(() => validateGoRun('', required));
  const emptyPackage = [{ Action: 'start', Package: 'attestara/bundle' },
    { Action: 'output', Package: 'attestara/bundle', Output: '? attestara/bundle [no test files]' },
    { Action: 'skip', Package: 'attestara/bundle' }];
  assert.deepEqual(validateGoRun(stream([...events, ...emptyPackage]), required), { packages: 1, passed: 1, skipped: 0 });
  assert.throws(() => validateGoRun(stream([...events, ...emptyPackage.map(event => ({ ...event, Package: 'attestara/tests' }))]), required));
});

test('C39 captures real nonzero exits and treats command arguments as inert strings', async t => {
  const root = await mkdtemp(join(tmpdir(), 'c39-command-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const result = await runCommand(process.execPath, ['-e', 'console.log(process.argv[1]); process.exit(7)', '$(touch must-not-exist)'],
    { cwd: root, log: join(root, 'failure.log') });
  assert.equal(result.exit_code, 7);
  assert.match(await readFile(join(root, 'failure.log'), 'utf8'), /\$\(touch must-not-exist\)/);
  await assert.rejects(readFile(join(root, 'must-not-exist')));
  assert.match(result.output_sha256, /^sha256:[a-f0-9]{64}$/);
});

test('C39 timeout, interruption and unavailable tools cannot produce a successful result', async t => {
  const root = await mkdtemp(join(tmpdir(), 'c39-interruption-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const timeout = await runCommand(process.execPath, ['-e', 'setInterval(() => {}, 1000)'],
    { cwd: root, log: join(root, 'timeout.log'), timeoutMs: 30 });
  assert.equal(timeout.timed_out, true);
  assert.notEqual(timeout.exit_code, 0);
  const controller = new AbortController();
  const interrupted = runCommand(process.execPath, ['-e', 'setInterval(() => {}, 1000)'],
    { cwd: root, log: join(root, 'interrupted.log'), signal: controller.signal });
  controller.abort();
  assert.equal((await interrupted).interrupted, true);
  const unavailable = await runCommand(join(root, 'missing-tool'), [], { cwd: root, log: join(root, 'missing.log') });
  assert.equal(unavailable.operational_error, 'ENOENT');
  assert.notEqual(unavailable.exit_code, 0);
});

test('C39 creates a fresh failed report instead of reusing previous success', async t => {
  const root = await mkdtemp(join(tmpdir(), 'c39-fresh-report-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  await mkdir(join(root, 'docs/engineering'), { recursive: true });
  await writeFile(join(root, 'docs/engineering/toolchain-lock.json'), JSON.stringify({ runtime: { node: process.version.slice(1), npm: '11.9.0' } }));
  await mkdir(join(root, 'dist/release-gate/previous'), { recursive: true });
  await writeFile(join(root, 'dist/release-gate/previous/report.json'), '{"passed":true}');
  const prior = process.env.npm_execpath;
  delete process.env.npm_execpath;
  try { assert.equal(await runReleaseGate(root), 1); }
  finally { if (prior !== undefined) process.env.npm_execpath = prior; }
  const directories = await readdir(join(root, 'dist/release-gate'));
  const reports = await Promise.all(directories.filter(name => name !== 'previous').map(name =>
    readFile(join(root, 'dist/release-gate', name, 'report.json'), 'utf8').then(JSON.parse)));
  assert.equal(reports.length, 1);
  assert.equal(reports[0].passed, false);
  assert.equal(reports[0].source, null);
  assert.match(reports[0].blockers[0], /pinned npm/);
});
