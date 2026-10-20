import assert from 'node:assert/strict';
import { execFileSync, spawn } from 'node:child_process';
import { mkdtemp, mkdir, readFile, writeFile, rm, cp, access, readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { once } from 'node:events';
import { createServer } from 'node:net';
import { chromium } from '@playwright/test';
import { runWorkflow, restoreWorkflow, engineCommand } from './clone-workflow.mjs';
import { inventory, sha256, publish, verifyInventory } from './package-release.mjs';
import { runUpgradeRecovery } from '../tests/release/upgrade-recovery.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const npm = process.env.npm_execpath;
assert.ok(npm, 'Invoke through pinned npm run test:package');
const run = (command, args, cwd, options = {}) => (execFileSync(command, args, { cwd, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 600000, ...options }) ?? '').trim();
const temporary = await mkdtemp(join(tmpdir(), 'attestara-c31-'));
const checks = [];
let service, releaseVerification;
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
try {
  const env = { ...process.env, GIT_INDEX_FILE: join(temporary, 'index'), GIT_AUTHOR_NAME: 'C31 test', GIT_AUTHOR_EMAIL: 'c31@example.invalid', GIT_COMMITTER_NAME: 'C31 test', GIT_COMMITTER_EMAIL: 'c31@example.invalid' };
  const epoch = run('git', ['show', '-s', '--format=%ct', 'HEAD'], root);
  env.GIT_AUTHOR_DATE = '@' + epoch + ' +0000'; env.GIT_COMMITTER_DATE = env.GIT_AUTHOR_DATE;
  run('git', ['read-tree', 'HEAD'], root, { env }); run('git', ['add', '-A'], root, { env });
  const tree = run('git', ['write-tree'], root, { env });
  const commit = run('git', ['commit-tree', tree, '-p', 'HEAD', '-m', 'temporary C31 candidate proof'], root, { env });
  const clones = [];
  for (const name of ['first', 'different-path']) {
    const checkout = join(temporary, name);
    run('git', ['clone', '--quiet', '--shared', '--no-checkout', root, checkout], temporary);
    run('git', ['checkout', '--quiet', '--detach', commit], checkout);
    clones.push(checkout);
  }
  const first = clones[0];
  for (const args of [['scripts/validate_repo.py'], ['-m', 'unittest', 'discover', '-s', 'tests/engineering', '-p', 'test_*.py', '-v'], ['scripts/verify_ci_source.py']]) run('python3', args, first, { stdio: 'inherit' });
  checks.push('actual contracts CI validators, engineering regressions and exact-checkout source integrity passed in clean candidate');
  // Errors must happen before any complete output is published.
  await writeFile(join(first, 'untracked-input'), 'dirty');
  assert.throws(() => run(process.execPath, [npm, 'run', 'package'], first), /clean source/);
  await rm(join(first, 'untracked-input'));
  const wrong = { ...process.env, npm_execpath: '/nonexistent/npm' };
  assert.throws(() => run(process.execPath, ['scripts/package-release.mjs'], first, { env: wrong }));
  checks.push('dirty source and incompatible/missing tool rejected');
  const missingLicenseRoot = join(temporary, 'missing-license');
  run('git', ['clone', '--quiet', '--shared', '--no-checkout', root, missingLicenseRoot], temporary);
  run('git', ['checkout', '--quiet', '--detach', commit], missingLicenseRoot);
  const license = join(missingLicenseRoot, 'docs/engineering/resources/go-LICENSE.txt');
  await rm(license);
  run('git', ['add', '-u', '--', 'docs/engineering/resources/go-LICENSE.txt'], missingLicenseRoot);
  run('git', ['-c', 'user.name=C31 test', '-c', 'user.email=c31@example.invalid', 'commit', '-m', 'temporary missing-license fixture'], missingLicenseRoot);
  assert.equal(run('git', ['status', '--porcelain'], missingLicenseRoot), '', 'missing-license fixture must be a clean source snapshot');
  assert.throws(
    () => run(process.execPath, [npm, 'run', 'package'], missingLicenseRoot),
    error => error.stderr?.includes('ENOENT: no such file or directory') && error.stderr.includes(license),
  );
  checks.push('clean source with a missing locked Go license is rejected before packaging');
  // Kill a real writer after staging exists; its private work must never be published.
  const writer = spawn(process.execPath, [npm, 'run', 'package'], { cwd: first, env: process.env, detached: true, stdio: ['ignore', 'ignore', 'pipe'] });
  let writerError = ''; writer.stderr.on('data', bytes => { writerError += bytes; });
  let staged = false;
  for (let i = 0; i < 300; i++) {
    try { staged = (await readdir(join(first, 'dist/packages'))).some(name => name.startsWith('.c31-')); } catch {}
    if (staged) break;
    if (writer.exitCode !== null) break;
    await new Promise(accept => setTimeout(accept, 20));
  }
  assert.ok(staged, 'writer reached staging: ' + writerError);
  const ended = once(writer, 'close'); process.kill(-writer.pid, 'SIGKILL'); await ended;
  await assert.rejects(access(join(first, 'dist/packages', commit)));
  checks.push('real SIGKILL before publication exposes no partial release');
  for (const checkout of clones) run(process.execPath, [npm, 'run', 'package'], checkout, { stdio: 'inherit' });
  const output = join(first, 'dist/packages', commit);
  const files = await inventory(output);
  for (const file of files) assert.equal(sha256(await readFile(join(clones[1], 'dist/packages', commit, file.path))), file.sha256, 'two-path reproduction: ' + file.path);
  checks.push('two independently installed/built paths reproduce every output byte');
  assert.throws(() => run(process.execPath, [npm, 'run', 'package'], first), /destination exists/);
  const sourceRoot = join(temporary, 'rebuild-source'); await mkdir(sourceRoot);
  run('tar', ['-xzf', join(output, 'attestara-' + commit + '-source.tar.gz'), '-C', sourceRoot], temporary);
  run(process.execPath, [npm, 'run', 'package'], sourceRoot, { stdio: 'inherit' });
  for (const file of files) assert.equal(sha256(await readFile(join(sourceRoot, 'dist/packages', commit, file.path))), file.sha256, 'source archive reproduction: ' + file.path);
  checks.push('source archive without Git rebuilds byte-identical artifacts');
  const sourceManifest = JSON.parse(await readFile(join(sourceRoot, 'SOURCE_MANIFEST.json')));
  assert.ok(sourceManifest.excluded.length > 0);
  assert.ok(sourceManifest.files.some(file => file.path === 'cmd/attestara/main.go'));
  assert.ok(sourceManifest.files.every(file => !file.path.includes('node_modules/') && !file.path.startsWith('.local-data/')));
  const changedPath = join(sourceRoot, sourceManifest.files[0].path), unchanged = await readFile(changedPath);
  await writeFile(changedPath, 'tampered');
  assert.throws(() => run(process.execPath, [npm, 'run', 'package'], sourceRoot), /checksum/);
  await writeFile(changedPath, unchanged);
  checks.push('distributed source tampering rejected; exclusions explicit');
  const runtime = join(temporary, 'installed'); await mkdir(runtime);
  run('tar', ['-xzf', join(output, 'attestara-' + commit + '-linux-x64.tar.gz'), '-C', runtime], temporary);
  run('sha256sum', ['-c', 'SHA256SUMS.txt'], runtime);
  await assert.rejects(access(join(runtime, 'node_modules')));
  const bom = JSON.parse(await readFile(join(runtime, 'sbom.cdx.json')));
  assert.equal(bom.specVersion, '1.7');
  for (const name of ['react', 'react-dom', 'scheduler', 'github.com/santhosh-tekuri/jsonschema/v6', 'golang.org/x/text', 'go', 'cyclonedx']) assert.ok(bom.components.some(component => component.name === name));
  const work = join(temporary, 'workflow'); await mkdir(work);
  const store = join(work, 'store');
  await start(runtime, store);
  const state = runWorkflow(runtime, work, store);
  const inert = join(work, 'inert'); await cp(join(runtime, 'tests/fixtures/attestara/C30'), inert, { recursive: true });
  const payload = Buffer.from('#!/bin/sh\ntouch ' + join(work, 'must-not-exist') + '\n');
  await writeFile(join(inert, 'subject.txt'), payload);
  const hostile = JSON.parse(await readFile(join(inert, 'manifest.json')));
  hostile.subject_digest = 'sha256:' + sha256(payload);
  const subjectFile = hostile.files.find(file => file.path === 'subject.txt');
  subjectFile.digest = hostile.subject_digest; subjectFile.size_bytes = payload.length;
  await writeFile(join(inert, 'manifest.json'), JSON.stringify(hostile));
  engineCommand(runtime, ['evaluate', '--input', join(inert, 'manifest.json'), '--files', inert, '--policy', join(inert, 'rule.json'), '--evidence', join(inert, 'evidence.json'), '--freshness', 'fresh']);
  await assert.rejects(access(join(work, 'must-not-exist')));
  checks.push('shell-like evidence remains inert in actual packaged engine');
  await stop(); const url = await start(runtime, store);
  const restored = restoreWorkflow(runtime, work, state);
  const browser = await chromium.launch({ ...(process.env.WEB_TEST_CHROMIUM ? { executablePath: process.env.WEB_TEST_CHROMIUM } : {}), args: ['--no-sandbox'] });
  try {
    const page = await browser.newPage({ viewport: { width: 360, height: 800 } });
    const writes = []; page.on('request', request => { if (request.method() !== 'GET') writes.push(request.url()); });
    await page.goto(url + '/#passport');
    await page.getByLabel('Choose passport file').setInputFiles({ name: 'restored.json', mimeType: 'application/json', buffer: Buffer.from(restored) });
    await page.getByRole('heading', { name: JSON.parse(restored).release_id }).waitFor();
    await page.getByText('Viewer assessment: Unverified').waitFor();
    assert.deepEqual(writes, []);
    console.log('Packaged browser: ' + browser.version());
    await stop();
    releaseVerification = await runUpgradeRecovery({ root, work: join(temporary, 'upgrade'), candidateArtifacts: output, candidateCommit: commit, runtime, start, stop, browser });
    checks.push('C35 real signed artifact verification and installed C34 -> candidate -> C34 upgrade/rollback preserve newer accepted data');
  } finally { await browser.close(); }
  await stop();
  checks.push('packaged CLI/full C30 negative workflow, service restart, restore and real browser parity without npm/Go on PATH');
  // Retain the tested local artifacts; no publication, branch or user index change.
  const artifacts = join(root, 'dist/packages'); await mkdir(artifacts, { recursive: true });
  const target = join(artifacts, commit);
  try { await access(target); } catch {
    const retained = await mkdtemp(join(artifacts, '.tested-')); await cp(output, retained, { recursive: true }); await publish(retained, target);
  }
  await verifyInventory(target, files);
  const proofDirectory = join(root, 'dist/release-verification'); await mkdir(proofDirectory, { recursive: true });
  await writeFile(join(proofDirectory, commit + '.json'), JSON.stringify(releaseVerification, null, 2) + '\n');
  console.log(JSON.stringify({ passed: true, candidateCommit: commit, candidateTree: tree, artifacts: target, checks, files, publication: 'local unsigned artifacts only', testEnvironment: 'three fresh source/build paths on this Linux host; not independent-host C36 evidence' }, null, 2));
} catch (error) { console.error('C31 integration failure:', error); throw error; }
finally { await stop().catch(error => console.error('C31 cleanup:', error.message)); await rm(temporary, { recursive: true, force: true }); }
