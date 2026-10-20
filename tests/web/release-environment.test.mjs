import assert from 'node:assert/strict';
import { test } from 'node:test';
import { generateKeyPairSync } from 'node:crypto';
import { mkdtemp, writeFile, rm, chmod, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { digest, findPreparedRun } from '../../scripts/release-publication.mjs';
import { assertReleaseEnvironment, readSigningKey, configureReleaseEnvironment, runGh } from '../../scripts/configure-release-environment.mjs';

const commit = 'a'.repeat(40);
const prepared = { id: 10, path: '.github/workflows/release.yml', head_sha: commit, head_branch: 'main', event: 'workflow_dispatch', display_title: `release ${commit} / publish=false`, status: 'completed', conclusion: 'success' };
test('publication requires the latest successful unsigned preparation of the same SHA', () => {
  assert.equal(findPreparedRun(commit, [prepared], '11').id, 10);
  for (const mutation of [{ head_sha: 'b'.repeat(40) }, { head_branch: 'topic' }, { event: 'push' }, { display_title: `release ${commit} / publish=true` }, { status: 'in_progress' }, { conclusion: 'failure' }]) {
    assert.throws(() => findPreparedRun(commit, [{ ...prepared, ...mutation }], '11'), /preparation/i);
  }
  assert.throws(() => findPreparedRun(commit, [prepared], '10'), /preparation/i);
  assert.throws(() => findPreparedRun(commit, [prepared, { ...prepared, id: 12, conclusion: 'failure' }], '13'), /preparation/i);
});

const environment = { name: 'release', deployment_branch_policy: { protected_branches: false, custom_branch_policies: true }, protection_rules: [{ type: 'required_reviewers', prevent_self_review: false, reviewers: [{ type: 'User', reviewer: { id: 111167585 } }] }] };
const policies = [{ id: 1, name: 'main', type: 'branch' }];
test('Environment verification requires its reviewer, main branch only and signing Secret metadata', () => {
  const check = extra => assertReleaseEnvironment({ environment, policies, reviewerId: 111167585, secretPresent: true, ...extra });
  assert.doesNotThrow(() => check());
  for (const extra of [{ environment: null }, { environment: { ...environment, protection_rules: [] } }, { environment: { ...environment, deployment_branch_policy: null } }, { policies: [] }, { policies: [{ ...policies[0], type: 'tag' }] }, { policies: [...policies, { id: 2, name: '*', type: 'branch' }] }, { secretPresent: false }, { reviewerId: 3 }]) assert.throws(() => check(extra));
});

async function keyFixture(t) {
  const directory = await mkdtemp(join(tmpdir(), 'attestara-owner-key-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const keys = generateKeyPairSync('ed25519');
  const publicKey = Buffer.from(keys.publicKey.export({ format: 'jwk' }).x, 'base64url').toString('hex');
  const keyPath = join(directory, 'private.pem');
  await writeFile(keyPath, keys.privateKey.export({ type: 'pkcs8', format: 'pem' }), { mode: 0o600 });
  const manifest = { schema_version: 1, repository: 'Parsa-Emami/Attestara', version: '1.0.0', tag: 'v1.0.0', profile: 'local-cli-snapshots', platform: 'linux-x64', signer_id: 'attestara-release-v1', public_key_hex: publicKey, fingerprint: digest(Buffer.from(publicKey, 'hex')) };
  return { directory, keyPath, manifest };
}
test('owner setup only reads the existing confined key with the pinned public identity', async t => {
  const f = await keyFixture(t);
  const bytes = await readSigningKey({ ...f, root: process.cwd() });
  assert.ok(bytes.length > 0); bytes.fill(0);
  await assert.rejects(readSigningKey({ ...f, root: process.cwd(), keyPath: join(f.directory, 'missing') }), /unavailable/i);
  await assert.rejects(readSigningKey({ ...f, root: f.directory }), /outside/i);
  await assert.rejects(readSigningKey({ ...f, root: process.cwd(), manifest: { ...f.manifest, public_key_hex: '0'.repeat(64), fingerprint: digest(Buffer.alloc(32)) } }), /pinned/i);
  await chmod(f.keyPath, 0o644);
  await assert.rejects(readSigningKey({ ...f, root: process.cwd() }), /private permissions/i);
  await chmod(f.keyPath, 0o600);
  const link = join(f.directory, 'link.pem'); await symlink(f.keyPath, link);
  await assert.rejects(readSigningKey({ ...f, root: process.cwd(), keyPath: link }), /unavailable/i);
});

test('missing original key causes zero remote operations', async () => {
  let calls = 0;
  await assert.rejects(configureReleaseEnvironment({ root: process.cwd(), apply: true, keyPath: join(tmpdir(), 'attestara-no-such-private-key.pem'), run: () => { calls++; } }), /unavailable/i);
  assert.equal(calls, 0);
});

test('CLI errors cannot print secret input or captured subprocess output', () => {
  const sensitive = 'synthetic secret input';
  assert.throws(() => runGh(['secret', 'set'], { input: sensitive }, () => { throw new Error(sensitive); }), error => !error.message.includes(sensitive));
});

test('owner setup uses stdin for the key and re-reads actual policy/secret metadata', async t => {
  const f = await keyFixture(t);
  const root = join(f.directory, 'checkout');
  const { mkdir } = await import('node:fs/promises');
  await mkdir(join(root, 'docs/release'), { recursive: true });
  await writeFile(join(root, 'docs/release/RELEASE_MANIFEST.json'), JSON.stringify(f.manifest));
  let current = null, rules = [{ id: 2, name: '*', type: 'tag' }], secret = false, writes = 0;
  let privateInput;
  const run = (args, options = {}) => {
    assert.equal(args.some(arg => String(arg).includes('PRIVATE KEY')), false);
    if (args[0] === 'secret') { privateInput = options.input; secret = true; writes++; return ''; }
    const method = args[args.indexOf('--method') + 1];
    const path = args[args.indexOf('--method') + 2];
    const body = options.input ? JSON.parse(options.input) : null;
    if (method !== 'GET') writes++;
    if (path === '/user') return JSON.stringify({ login: 'Parsa-Emami' });
    if (path === '/users/Parsa-Emami') return JSON.stringify({ id: 111167585 });
    if (path.endsWith('/collaborators/Parsa-Emami/permission')) return JSON.stringify({ permission: 'admin' });
    if (path === '/repos/Parsa-Emami/Attestara') return JSON.stringify({ permissions: { admin: true } });
    if (path.includes('/environments?')) return JSON.stringify({ total_count: current ? 1 : 0, environments: current ? [current] : [] });
    if (path.endsWith('/environments/release') && method === 'PUT') { current = { ...environment, ...body, protection_rules: environment.protection_rules }; return JSON.stringify(current); }
    if (path.includes('/deployment-branch-policies?')) return JSON.stringify({ total_count: rules.length, branch_policies: rules });
    if (path.endsWith('/deployment-branch-policies') && method === 'POST') { rules.push({ id: 1, ...body }); return JSON.stringify(rules.at(-1)); }
    if (path.endsWith('/deployment-branch-policies/2') && method === 'DELETE') { rules = rules.filter(r => r.id !== 2); return ''; }
    if (path.includes('/secrets?')) return JSON.stringify({ total_count: secret ? 1 : 0, secrets: secret ? [{ name: 'ATTESTARA_RELEASE_PRIVATE_KEY' }] : [] });
    throw new Error('Unexpected test request');
  };
  const result = await configureReleaseEnvironment({ root, apply: true, keyPath: f.keyPath, run });
  assert.equal(result.verified, true); assert.equal(result.secret_value_verified, false);
  assert.deepEqual(rules, policies); assert.ok(writes >= 4);
  assert.ok(privateInput.every(byte => byte === 0), 'key buffer cleared after subprocess finishes');
  writes = 0;
  await configureReleaseEnvironment({ root, run });
  assert.equal(writes, 0, 'default check never mutates the repository');
});
