import assert from 'node:assert/strict';
import { test } from 'node:test';
import { generateKeyPairSync, createHash } from 'node:crypto';
import { mkdtemp, writeFile, readFile, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { assertCandidate, signChecksums, publishRelease, downloadRelease, validateManifest, createReleaseKey, digest } from '../../scripts/release-publication.mjs';

const commit = 'a'.repeat(40);
const paths = ['contracts.yml', 'plan.yml', 'runtime.yml', 'web.yml', 'release-gate.yml'];
const runs = paths.map((name, index) => ({ id: index + 1, path: '.github/workflows/' + name, head_sha: commit, head_branch: 'main', event: 'push', status: 'completed', conclusion: 'success' }));
const report = { schema_version: 1, planning_template: false, task: 'C39', profile: 'local-cli-snapshots', passed: true, run: { id: process.env.GITHUB_RUN_ID ?? 'synthetic-c40', kind: 'github-actions' }, source: { commit }, results: { source_unchanged: true }, blockers: [], commands: Array.from({ length: 28 }, () => ({ exit_code: 0 })) };
test('C40 requires the latest successful five workflows and exact main/gate source', () => {
  const candidate = { commit, main: commit, runs, report, tagExists: false, releaseExists: false };
  assert.doesNotThrow(() => assertCandidate(candidate));
  for (const mutation of [ { main: 'b'.repeat(40) }, { runs: runs.slice(1) }, { tagExists: true }, { releaseExists: true }, { report: { ...report, source: { commit: 'b'.repeat(40) } } }, { report: { ...report, passed: false } }, { runs: [...runs, { ...runs[0], id: 100, conclusion: 'failure' }] }, { runs: [...runs, { ...runs[0], id: 100, status: 'in_progress' }] } ]) {
    assert.throws(() => assertCandidate({ ...candidate, ...mutation }));
  }
});

async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), 'attestara-c40-sign-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const keys = generateKeyPairSync('ed25519');
  const publicKey = Buffer.from(keys.publicKey.export({ format: 'jwk' }).x, 'base64url').toString('hex');
  const manifest = { schema_version: 1, repository: 'Parsa-Emami/Attestara', version: '1.0.0', tag: 'v1.0.0', profile: 'local-cli-snapshots', platform: 'linux-x64', signer_id: 'attestara-release-v1', public_key_hex: publicKey, fingerprint: 'sha256:' + createHash('sha256').update(Buffer.from(publicKey, 'hex')).digest('hex') };
  const files = { 'release.json': JSON.stringify({ schemaVersion: 1, sourceCommit: commit, sourceIdentity: 'sha256:' + 'c'.repeat(64), scope: manifest.profile, platform: manifest.platform }), 'sbom.cdx.json': '{}', ['attestara-' + commit + '-linux-x64.tar.gz']: 'synthetic runtime bytes; never executed', ['attestara-' + commit + '-source.tar.gz']: 'synthetic source bytes; never executed' };
  for (const [name, bytes] of Object.entries(files)) await writeFile(join(directory, name), bytes);
  const sums = Object.entries(files).map(([name, bytes]) => createHash('sha256').update(bytes).digest('hex') + '  ' + name).join('\n') + '\n';
  await writeFile(join(directory, 'SHA256SUMS.txt'), sums);
  return { directory, keys, manifest, files };
}

test('C40 signing reuses exact checksum bytes and refuses a wrong key or altered package', async t => {
  const f = await fixture(t);
  validateManifest(f.manifest);
  await assert.rejects(signChecksums({ ...f, commit, privateKey: generateKeyPairSync('ed25519').privateKey }), /key/i);
  const assessment = await signChecksums({ ...f, commit, privateKey: f.keys.privateKey });
  assert.equal(assessment.release_trusted, true);
  await assert.rejects(signChecksums({ ...f, commit, privateKey: f.keys.privateKey }), /exist/i);
  await writeFile(join(f.directory, 'sbom.cdx.json'), 'tampered');
  await assert.rejects(signChecksums({ ...f, commit, privateKey: f.keys.privateKey }));
});

test('C40 cannot publish without explicit authorization or after partial uploads', async t => {
  const f = await fixture(t);
  await signChecksums({ ...f, commit, privateKey: f.keys.privateKey });
  let writes = 0;
  const request = async (path, options = {}) => {
    if (!options.method) {
      if (path.endsWith('/branches/main')) return { commit: { sha: commit } };
      if (path.includes('/actions/runs?')) return { workflow_runs: runs, total_count: runs.length };
      if (path.includes('/releases?')) return [];
      return null;
    }
    writes++;
    if (path.endsWith('/releases')) return { id: 1, upload_url: 'https://uploads.github.com/repos/Parsa-Emami/Attestara/releases/1/assets{?name,label}' };
    return { digest: 'sha256:' + '0'.repeat(64), size: 0 };
  };
  await assert.rejects(publishRelease({ ...f, commit, report, request, authorized: false }));
  assert.equal(writes, 0);
  await assert.rejects(publishRelease({ ...f, commit, report, request, authorized: true }), /upload/i);
  assert.equal(writes, 2, 'failure leaves a draft and never publishes or deletes it');
});

test('C40 download refuses a draft, wrong tag/source and missing/unsafe assets before downloading', async t => {
  const f = await fixture(t);
  let downloaded = false;
  for (const release of [{ draft: true }, { draft: false, prerelease: true }, { draft: false, tag_name: 'v0' }, { draft: false, tag_name: 'v1.0.0', assets: [] }]) {
    await assert.rejects(downloadRelease({ ...f, commit, request: async () => release, fetchFile: async () => { downloaded = true; } }));
  }
  assert.equal(downloaded, false);
  assert.equal((await readFile(join(f.directory, 'release.json'), 'utf8')).includes(commit), true);
});

test('C40 complete publication and public re-download authenticate the same bytes; corrupt download fails', async t => {
  const f = await fixture(t);
  await signChecksums({ ...f, commit, privateKey: f.keys.privateKey });
  const uploaded = [];
  const release = { id: 7, draft: true, prerelease: false, tag_name: 'v1.0.0', assets: uploaded, html_url: 'https://github.com/Parsa-Emami/Attestara/releases/tag/v1.0.0' };
  let tag, published = false;
  const blobs = new Map();
  const request = async (path, options = {}) => {
    if (options.upload) {
      const name = new URL(path).searchParams.get('name');
      const asset = { id: uploaded.length + 1, name, digest: digest(options.body), size: options.body.length, state: 'uploaded', browser_download_url: 'https://github.com/Parsa-Emami/Attestara/releases/download/v1.0.0/' + name };
      uploaded.push(asset); blobs.set(asset.browser_download_url, options.body); return asset;
    }
    if (path.endsWith('/branches/main')) return { commit: { sha: commit } };
    if (path.includes('/actions/runs?')) return { total_count: runs.length, workflow_runs: runs };
    if (path.includes('/releases?')) return [];
    if (path.endsWith('/git/ref/tags/v1.0.0')) return tag;
    if (path.endsWith('/git/refs')) { tag = { object: { type: 'commit', sha: options.body.sha } }; return tag; }
    if (path.endsWith('/releases') && options.method === 'POST') return { ...release, upload_url: 'https://uploads.github.com/repos/Parsa-Emami/Attestara/releases/7/assets{?name,label}' };
    if (path.endsWith('/releases/7')) {
      if (options.method === 'PATCH') { published = true; release.draft = false; }
      return release;
    }
    if (path.endsWith('/releases/tags/v1.0.0')) return published ? release : null;
    throw new Error('Unexpected request');
  };
  const proof = await publishRelease({ ...f, commit, report, authorized: true, request });
  assert.equal(proof.published, true); assert.equal(uploaded.length, 6);
  const download = join(f.directory, 'download');
  const fetchFile = async (url, path) => writeFile(path, blobs.get(url), { flag: 'wx' });
  const result = await downloadRelease({ manifest: f.manifest, directory: download, commit, request, fetchFile });
  assert.equal(result.assessment.release_trusted, true);
  await assert.rejects(downloadRelease({ manifest: f.manifest, directory: join(f.directory, 'corrupt'), commit, request, fetchFile: async (url, path) => writeFile(path, 'corrupt') }), /digest/);
  const originalURL = uploaded[0].browser_download_url;
  uploaded[0].browser_download_url = 'http://127.0.0.1/private';
  await assert.rejects(downloadRelease({ manifest: f.manifest, directory: join(f.directory, 'unsafe'), commit, request, fetchFile }), /Unsafe/);
  uploaded[0].browser_download_url = originalURL;
  tag.object.sha = 'b'.repeat(40);
  await assert.rejects(downloadRelease({ manifest: f.manifest, directory: join(f.directory, 'wrong-source'), commit, request, fetchFile }), /source/);
});

test('C40 key initialization is private, external and never replaces an existing key directory', async t => {
  const parent = await mkdtemp(join(tmpdir(), 'attestara-c40-key-')); t.after(() => rm(parent, { recursive: true, force: true }));
  const directory = join(parent, 'key');
  const info = await createReleaseKey(directory, process.cwd());
  assert.match(info.public_key_hex, /^[a-f0-9]{64}$/);
  assert.equal((await stat(directory)).mode & 0o777, 0o700);
  assert.equal((await stat(join(directory, 'private.pem'))).mode & 0o777, 0o600);
  await assert.rejects(createReleaseKey(directory, process.cwd()), /exist/i);
  await assert.rejects(createReleaseKey(join(process.cwd(), 'private-key'), process.cwd()), /outside/);
});

test('C40 synthetic scenarios retain exact licensed provenance', async () => {
  const directory = new URL('../fixtures/attestara/C40/', import.meta.url);
  const provenance = JSON.parse(await readFile(new URL('provenance.json', directory)));
  assert.equal(provenance.license, 'Apache-2.0');
  for (const [name, hash] of Object.entries(provenance.files)) assert.equal(digest(await readFile(new URL(name, directory))), hash);
  const cases = JSON.parse(await readFile(new URL('cases.json', directory)));
  assert.deepEqual(cases.required_workflows, paths);
  assert.equal(cases.publication_without_authorization, 'rejected');
});
