import test from 'node:test';
import assert from 'node:assert/strict';
import { generateKeyPairSync, sign, createHash } from 'node:crypto';
import { mkdtemp, mkdir, writeFile, readFile, rm, symlink, chmod } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import { verifyRelease } from '../../scripts/verify-release.mjs';

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const commit = 'a'.repeat(40);
const hash = 'sha256:' + 'b'.repeat(64);
async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'attestara-c35-unit-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, 'release'); await mkdir(directory);
  const keys = generateKeyPairSync('ed25519');
  const key = Buffer.from(keys.publicKey.export({ format: 'jwk' }).x, 'base64url').toString('hex');
  const signature = join(root, 'signature.json'), trust = join(root, 'trust.json');
  await writeFile(trust, JSON.stringify({ schema_version: '1', trusted_signers: { 'c35-test': key } }));
  const files = {
    ['attestara-' + commit + '-linux-x64.tar.gz']: Buffer.from('#!/bin/sh\nexit 99\n'),
    ['attestara-' + commit + '-source.tar.gz']: Buffer.from('inert synthetic archive bytes'),
    'sbom.cdx.json': Buffer.from('{}'),
    'release.json': Buffer.from(JSON.stringify({ schemaVersion: 1, sourceCommit: commit, sourceIdentity: hash, platform: 'linux-x64', scope: 'local-cli-snapshots' })),
  };
  for (const [name, bytes] of Object.entries(files)) await writeFile(join(directory, name), bytes);
  const sums = Object.entries(files).sort(([a], [b]) => a.localeCompare(b)).map(([name, bytes]) => sha256(bytes) + '  ' + name).join('\n') + '\n';
  async function signed(bytes = sums, changes = {}) {
    await writeFile(join(directory, 'SHA256SUMS.txt'), bytes);
    await writeFile(signature, JSON.stringify({ schema_version: '1', signer_id: 'c35-test', public_key_hex: key, signature_hex: sign(null, Buffer.from(bytes), keys.privateKey).toString('hex'), ...changes }));
  }
  await signed();
  return { root, directory, signature, trust, expectedSourceCommit: commit, freshness: 'fresh', files, sums, signed };
}

test('C35 authenticates exact checksum bytes and independent signer without executing artifacts', async t => {
  const f = await fixture(t);
  const result = await verifyRelease(f);
  assert.equal(result.exitCode, 0);
  assert.equal(result.assessment.release_trusted, true);
  assert.equal(result.assessment.cryptographic_valid, true);
  assert.equal(result.assessment.signer_trusted, true);
  assert.equal(result.assessment.file_integrity, 'verified');
  assert.deepEqual(await verifyRelease(f), result, 'repeat is a stateless assessment');
  const run = spawnSync(process.execPath, ['scripts/verify-release.mjs', '--directory', f.directory, '--signature', f.signature, '--trust', f.trust, '--expected-source-commit', commit, '--freshness', 'fresh'], { encoding: 'utf8' });
  assert.equal(run.status, 0, run.stderr);
  assert.deepEqual(JSON.parse(run.stdout), result.assessment);
});

test('C35 distinguishes unsigned, untrusted, wrong-key and stale assessments', async t => {
  const f = await fixture(t);
  for (const options of [{ signature: undefined }, { freshness: 'stale' }]) {
    const r = await verifyRelease({ ...f, ...options });
    assert.equal(r.exitCode, 1); assert.equal(r.assessment.release_trusted, false);
  }
  for (const signers of [{}, { 'c35-test': '00'.repeat(32) }]) {
    await writeFile(f.trust, JSON.stringify({ schema_version: '1', trusted_signers: signers }));
    const r = await verifyRelease(f);
    assert.equal(r.exitCode, 1); assert.equal(r.assessment.cryptographic_valid, true); assert.equal(r.assessment.signer_trusted, false);
  }
  await f.signed(f.sums, { signer_id: 'another-signer' });
  assert.equal((await verifyRelease(f)).exitCode, 1);
});

test('C35 rejects a modified byte and every missing required output', async t => {
  const f = await fixture(t);
  for (const [name, bytes] of Object.entries(f.files)) {
    const path = join(f.directory, name);
    await writeFile(path, Buffer.concat([bytes, Buffer.from('x')]));
    assert.equal((await verifyRelease(f)).exitCode, 2, name);
    await rm(path);
    assert.equal((await verifyRelease(f)).exitCode, 2, 'missing ' + name);
    await writeFile(path, bytes);
  }
  await rm(join(f.directory, 'SHA256SUMS.txt'));
  assert.equal((await verifyRelease(f)).exitCode, 2);
});

test('C35 rejects tampered signatures/checksums and wrong expected source', async t => {
  const f = await fixture(t);
  await f.signed(f.sums, { signature_hex: '00'.repeat(64) });
  assert.equal((await verifyRelease(f)).exitCode, 2);
  await f.signed();
  await writeFile(join(f.directory, 'SHA256SUMS.txt'), f.sums + '\n');
  assert.equal((await verifyRelease(f)).exitCode, 2);
  await f.signed();
  assert.equal((await verifyRelease({ ...f, expectedSourceCommit: 'c'.repeat(40) })).exitCode, 2);
  await f.signed(Buffer.from([0xff]));
  assert.equal((await verifyRelease(f)).exitCode, 2);
});

test('C35 rejects signed unsafe, duplicate, missing and extra checksum entries', async t => {
  const f = await fixture(t);
  for (const sums of [f.sums + f.sums.split('\n')[0] + '\n', f.sums.split('\n').slice(1).join('\n'), f.sums + '0'.repeat(64) + '  ../escape\n', f.sums + '0'.repeat(64) + '  extra\n', f.sums.replace('  sbom.cdx.json', '  /sbom.cdx.json'), f.sums.replace('  sbom.cdx.json', '  a\\b')]) {
    await f.signed(sums);
    assert.equal((await verifyRelease(f)).exitCode, 2);
  }
});

test('C35 bounds metadata, validates descriptors and refuses symbolic/special files', async t => {
  const f = await fixture(t);
  for (const path of [f.signature, f.trust, join(f.directory, 'SHA256SUMS.txt'), join(f.directory, 'release.json')]) {
    const original = await readFile(path);
    await writeFile(path, Buffer.alloc(65537, 32));
    assert.equal((await verifyRelease(f)).exitCode, 2);
    await writeFile(path, original);
    await rm(path); await symlink('/dev/zero', path);
    assert.equal((await verifyRelease(f)).exitCode, 2);
    await rm(path); await writeFile(path, original);
  }
  for (const changes of [{ public_key_hex: 'bad' }, { signature_hex: 'a' }, { schema_version: '2' }, { signer_id: '' }, { extra: true }]) {
    await f.signed(f.sums, changes);
    assert.equal((await verifyRelease(f)).exitCode, 2);
  }
  await f.signed();
  await writeFile(f.trust, JSON.stringify({ schema_version: '1', trusted_signers: { 'c35-test': 'bad' } }));
  assert.equal((await verifyRelease(f)).exitCode, 2);
  assert.equal((await verifyRelease({ ...f, freshness: undefined })).exitCode, 2);
  assert.equal((await verifyRelease({ ...f, trust: undefined })).exitCode, 2);
  await symlink(f.directory, join(f.root, 'linked'));
  assert.equal((await verifyRelease({ ...f, directory: join(f.root, 'linked') })).exitCode, 2);
  const detached = JSON.parse(await readFile(f.signature));
  await writeFile(f.trust, JSON.stringify({ schema_version: '1', trusted_signers: { 'c35-test': detached.public_key_hex } }));
  assert.equal((await verifyRelease(f)).exitCode, 0);
  const archive = join(f.directory, 'attestara-' + commit + '-source.tar.gz');
  await rm(archive);
  assert.equal(spawnSync('mkfifo', [archive]).status, 0);
  assert.equal((await verifyRelease(f)).exitCode, 2);
  const run = spawnSync(process.execPath, ['scripts/verify-release.mjs', '--unexpected'], { encoding: 'utf8' });
  assert.equal(run.status, 2); assert.equal(JSON.parse(run.stdout).release_trusted, false);
});

test('C35 reports unavailable trust files as an operational failure without leaking paths', async t => {
  const f = await fixture(t);
  await chmod(f.trust, 0);
  try {
    const r = await verifyRelease(f);
    assert.equal(r.exitCode, 3); assert.equal(r.assessment.release_trusted, false);
    assert.ok(!JSON.stringify(r).includes(f.root));
  } finally { await chmod(f.trust, 0o600); }
});

test('C35 rejects freshly signed incompatible release metadata', async t => {
  const f = await fixture(t);
  for (const changes of [{ schemaVersion: 2 }, { sourceCommit: 'c'.repeat(40) }, { sourceIdentity: 'bad' }, { platform: 'win32-x64' }, { scope: 'web-foundation' }]) {
    const bytes = JSON.stringify({ ...JSON.parse(f.files['release.json']), ...changes });
    await writeFile(join(f.directory, 'release.json'), bytes);
    await f.signed(f.sums.replace(sha256(f.files['release.json']), sha256(bytes)));
    const r = await verifyRelease(f);
    assert.equal(r.exitCode, 2); assert.equal(r.assessment.file_integrity, 'verified'); assert.equal(r.assessment.release_trusted, false);
  }
});

test('C35 fixture provenance records exact licensed scenario bytes', async () => {
  const root = new URL('../fixtures/attestara/C35/', import.meta.url);
  const provenance = JSON.parse(await readFile(new URL('provenance.json', root)));
  assert.equal(provenance.license, 'Apache-2.0');
  for (const [path, digest] of Object.entries(provenance.files)) assert.equal('sha256:' + sha256(await readFile(new URL(path, root))), digest);
});
