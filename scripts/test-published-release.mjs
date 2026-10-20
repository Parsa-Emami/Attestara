import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdir, mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { parseArgs } from 'node:util';
import { chromium } from '@playwright/test';
import { assessRelease, downloadRelease, digest } from './release-publication.mjs';
import { safeName, inventory } from './package-release.mjs';
import { installedRuntime } from './installed-runtime.mjs';
import { runWorkflow, restoreWorkflow, engineCommand, workflowCommands } from './clone-workflow.mjs';
import { runUpgradeRecovery } from '../tests/release/upgrade-recovery.mjs';

export async function testPublishedRelease({ root, commit, localDirectory }) {
  assert.match(commit, /^[a-f0-9]{40}$/);
  const manifest = JSON.parse(await readFile(join(root, 'docs/release/RELEASE_MANIFEST.json')));
  const reports = join(root, 'dist/published-release'); await mkdir(reports, { recursive: true });
  const output = await mkdtemp(join(reports, 'run-'));
  const work = await mkdtemp(join(tmpdir(), 'attestara-c40-install-'));
  const session = installedRuntime();
  const record = { schema_version: 1, task: 'C40', passed: false, published: localDirectory ? false : null, source_commit: commit, tag: manifest.tag, profile: manifest.profile, started_at: new Date().toISOString(), mode: localDirectory ? 'local-rehearsal' : 'public-release', node: process.version, signer_fingerprint: manifest.fingerprint, checks: [], blockers: [] };
  let browser;
  try {
    const directory = localDirectory ? resolve(localDirectory) : join(work, 'download');
    if (!localDirectory) {
      record.publication = await downloadRelease({ directory, commit, manifest });
      record.published = true;
    }
    const result = await assessRelease({ directory, commit, manifest });
    assert.equal(result.exitCode, 0, JSON.stringify(result.assessment));
    record.authenticity = result.assessment;
    record.artifacts = await inventory(directory);
    const archive = join(directory, 'attestara-' + commit + '-linux-x64.tar.gz');
    const run = (command, args, cwd = root) => execFileSync(command, args, { cwd, encoding: 'utf8', timeout: 600000, maxBuffer: 16 * 1024 * 1024 });
    const names = run('tar', ['-tzf', archive]).trimEnd().split('\n');
    names.forEach(safeName);
    assert.ok(run('tar', ['-tvzf', archive]).trimEnd().split('\n').every(line => line.startsWith('-')), 'Runtime archive contains only regular files');
    const runtime = join(work, 'installed'); await mkdir(runtime);
    run('tar', ['-xzf', archive, '--no-same-owner', '--no-same-permissions', '-C', runtime]);
    run('sha256sum', ['-c', 'SHA256SUMS.txt'], runtime);
    assert.deepEqual(JSON.parse(await readFile(join(runtime, 'docs/release/RELEASE_MANIFEST.json'))), manifest);
    assert.ok((await readFile(join(runtime, 'docs/release/RELEASING.md'))).length > 0);
    record.checks.push('Maintainer signature, expected source, all downloaded bytes and extracted runtime checksums verified before execution.');
    const workflow = join(work, 'workflow'); await mkdir(workflow);
    const store = join(workflow, 'store');
    await session.start(runtime, store);
    const state = runWorkflow(runtime, workflow, store);
    await session.stop();
    const url = await session.start(runtime, store);
    assert.equal(engineCommand(runtime, ['verify', '--store', store, '--operation-id', 'release-1']).stdout, state.first);
    const restored = restoreWorkflow(runtime, workflow, state);
    browser = await chromium.launch({ ...(process.env.WEB_TEST_CHROMIUM ? { executablePath: process.env.WEB_TEST_CHROMIUM } : {}), args: ['--no-sandbox'] });
    const page = await browser.newPage({ viewport: { width: 360, height: 800 } });
    const writes = []; page.on('request', request => { if (request.method() !== 'GET') writes.push(request.url()); });
    await page.goto(url + '/#passport');
    await page.getByLabel('Choose passport file').setInputFiles({ name: 'restored.json', mimeType: 'application/json', buffer: Buffer.from(restored) });
    await page.getByRole('heading', { name: JSON.parse(restored).release_id }).waitFor();
    await page.getByText('Viewer assessment: Unverified').waitFor();
    assert.deepEqual(writes, []);
    await page.close(); await session.stop();
    record.checks.push('Installed CLI reference workflow, replay/conflict, restart, new-store restore and 360px local viewer passed without Go/npm on the service PATH.');
    // C35 builds the fixed C34 baseline; candidate bytes remain the authenticated download.
    record.recovery = await runUpgradeRecovery({ root, work: join(work, 'upgrade'), candidateArtifacts: directory, candidateCommit: commit, runtime, start: session.start, stop: session.stop, browser });
    record.checks.push('C34 -> published candidate -> C34 rollback retains newer accepted data; old backup cannot overwrite it, and shell-like input remains inert.');
    record.passport_sha256 = digest(Buffer.from(restored));
    record.browser = browser.version();
    record.commands = workflowCommands;
    record.passed = true;
  } catch (error) { record.blockers.push(error.message); }
  finally {
    await session.stop().catch(error => { record.passed = false; record.blockers.push(error.message); });
    if (browser) await browser.close().catch(error => { record.passed = false; record.blockers.push(error.message); });
    record.finished_at = new Date().toISOString();
    record.limits = ['Linux x86_64 single operator only; not a new resource-capacity certificate.', 'C35 recovery uses synthetic signatures for its fixed baseline comparison only; maintainer authenticity is assessed separately before candidate extraction.', ...(localDirectory ? ['Local rehearsal is not published installation proof.'] : [])];
    await writeFile(join(output, 'report.json'), JSON.stringify(record, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
    await rm(work, { recursive: true, force: true });
  }
  return { passed: record.passed, published: record.published, report: join(output, 'report.json') };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const { values } = parseArgs({ options: { 'source-commit': { type: 'string' }, 'local-directory': { type: 'string' } } });
    const result = await testPublishedRelease({ root: fileURLToPath(new URL('../', import.meta.url)), commit: values['source-commit'], localDirectory: values['local-directory'] });
    console.log(JSON.stringify(result)); process.exitCode = result.passed ? 0 : 1;
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
