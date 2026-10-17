import test from 'node:test';
import assert from 'node:assert/strict';
import { cp, mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { verifyResources, verifyArtifact } from '../../scripts/verify-resources.mjs';

const sourceRoot = fileURLToPath(new URL('../../', import.meta.url));
async function copyLock(t) {
  const root = await mkdtemp(join(tmpdir(), 'attestara-c02-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const path of ['package.json', 'package-lock.json', 'LICENSE', 'THIRD_PARTY_NOTICES.md', 'ops/runtime-requirements.json', 'docs/engineering/toolchain-lock.json', 'docs/engineering/resources', 'tests/fixtures/attestara/C01', 'tests/fixtures/attestara/C02']) {
    await cp(join(sourceRoot, path), join(root, path), { recursive: true });
  }
  return root;
}
async function mutate(root, change) {
  const path = join(root, 'docs/engineering/toolchain-lock.json');
  const lock = JSON.parse(await readFile(path, 'utf8'));
  change(lock);
  await writeFile(path, JSON.stringify(lock));
}

test('C02 validates pinned resources offline without requiring optional providers or a product engine', async t => {
  const result = await verifyResources({ root: await copyLock(t) });
  assert.ok(result.resources >= 7);
  assert.equal(result.engineExecutionVerified, false);
});

test('C02 rejects mutable pins, missing authority/license, removed prerequisites and unsupported mandatory platforms', async t => {
  const cases = JSON.parse(await readFile(join(sourceRoot, 'tests/fixtures/attestara/C02/negative.json'), 'utf8'));
  for (const { field, value, error } of cases.cases) {
    const root = await copyLock(t);
    await mutate(root, lock => {
      const go = lock.resources.find(r => r.id === 'go');
      if (field === 'remove-go') lock.resources = lock.resources.filter(r => r.id !== 'go');
      else go[field] = value;
    });
    await assert.rejects(verifyResources({ root }), new RegExp(error, 'i'));
  }
  await assert.rejects(verifyResources({ root: await copyLock(t), platform: 'darwin-arm64' }), /platform/i);
});

test('C02 rejects tampered checksum evidence, dependency locks, notices and escaping snapshot paths', async t => {
  for (const target of ['package-lock.json', 'THIRD_PARTY_NOTICES.md', 'docs/engineering/resources/go-release.json']) {
    const root = await copyLock(t);
    await writeFile(join(root, target), 'tampered');
    await assert.rejects(verifyResources({ root }), /checksum|digest|JSON/i);
  }
  const root = await copyLock(t);
  await mutate(root, lock => { lock.resources[0].license.path = '../outside'; });
  await assert.rejects(verifyResources({ root }), /path/i);
  const linkRoot = await copyLock(t);
  const target = join(linkRoot, 'docs/engineering/resources/go-release.json');
  await rm(target);
  await symlink('/etc/hosts', target);
  await assert.rejects(verifyResources({ root: linkRoot }), /path/i);
});

test('C02 verifies artifact bytes before accepting them and rejects a one-byte mutation', async t => {
  const root = await copyLock(t);
  const fixture = JSON.parse(await readFile(join(root, 'tests/fixtures/attestara/C02/positive.json'), 'utf8'));
  const expected = JSON.parse(await readFile(join(root, 'tests/fixtures/attestara/C02/expected.json'), 'utf8'));
  const path = join(root, 'artifact');
  await writeFile(path, fixture.bytes_utf8);
  await verifyArtifact(path, expected.sha256);
  await writeFile(path, fixture.bytes_utf8 + '!');
  await assert.rejects(verifyArtifact(path, expected.sha256), /checksum/i);
});

test('C02 compares pins with separately published checksums and fails on absent required artifacts', async t => {
  const root = await copyLock(t);
  const path = 'docs/engineering/resources/opa-checksums.txt';
  const bytes = '0'.repeat(64) + '  opa_linux_amd64_static\n';
  await writeFile(join(root, path), bytes);
  await mutate(root, lock => { lock.resources.find(r => r.id === 'opa').integrity.sha256 = createHash('sha256').update(bytes).digest('hex'); });
  await assert.rejects(verifyResources({ root }), /published checksum/i);
  const missing = await mkdtemp(join(tmpdir(), 'attestara-c02-artifacts-'));
  t.after(() => rm(missing, { recursive: true, force: true }));
  await assert.rejects(verifyResources({ root: await copyLock(t), artifacts: missing }), /unavailable required artifact/i);
});
