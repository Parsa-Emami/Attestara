import { execFileSync } from 'node:child_process';
import { createPrivateKey, createPublicKey } from 'node:crypto';
import { constants } from 'node:fs';
import { open, readFile, realpath, stat } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join, resolve, relative, isAbsolute, dirname, basename } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { parseArgs } from 'node:util';
import { validateManifest } from './release-publication.mjs';

const prefix = '/repos/Parsa-Emami/Attestara';
const environmentPath = prefix + '/environments/release';
const secretName = 'ATTESTARA_RELEASE_PRIVATE_KEY';
const check = (condition, message) => { if (!condition) throw new Error(message); };

// Capture and discard errors: gh diagnostics must never echo signing material.
export function runGh(args, options = {}, execute = execFileSync) {
  try { return execute('gh', args, { ...options, encoding: 'utf8', timeout: 120000, maxBuffer: 4 * 1024 * 1024, stdio: ['pipe', 'pipe', 'pipe'] }); }
  catch { throw new Error('GitHub CLI operation failed; check gh installation, github.com authentication and repository administration permissions. No subprocess output was disclosed.'); }
}

export async function readSigningKey({ root, keyPath, manifest }) {
  let directory;
  try { directory = await realpath(dirname(keyPath)); }
  catch { throw new Error('Original release signing key unavailable; do not generate a replacement or paste a key into chat.'); }
  const rel = relative(await realpath(root), join(directory, basename(keyPath)));
  check(rel.startsWith('../') || isAbsolute(rel), 'Signing key must remain outside the repository');
  let handle, bytes;
  try {
    try { handle = await open(join(directory, basename(keyPath)), constants.O_RDONLY | constants.O_NOFOLLOW); }
    catch { throw new Error('Original release signing key unavailable; do not generate a replacement or paste a key into chat.'); }
    const info = await handle.stat();
    check(info.isFile() && info.size > 0 && info.size <= 16384 && info.nlink === 1, 'Invalid signing key file');
    check((info.mode & 0o077) === 0 && (typeof process.getuid !== 'function' || info.uid === process.getuid()), 'Signing key requires owner-only private permissions');
    check(((await stat(directory)).mode & 0o077) === 0, 'Signing key directory requires owner-only private permissions');
    bytes = await handle.readFile();
    let key;
    try { key = createPrivateKey({ key: bytes, format: 'pem', type: 'pkcs8' }); }
    catch { throw new Error('Original release signing key is invalid'); }
    check(key.asymmetricKeyType === 'ed25519', 'Require the existing Ed25519 signing key');
    const publicKey = Buffer.from(createPublicKey(key).export({ format: 'jwk' }).x, 'base64url').toString('hex');
    check(publicKey === manifest.public_key_hex, 'Private key differs from the independently pinned release key');
    return bytes;
  } catch (error) { bytes?.fill(0); throw error; }
  finally { await handle?.close(); }
}

export function assertReleaseEnvironment({ environment, policies, reviewerId, secretPresent, requireSecret = true }) {
  check(environment?.name === 'release', 'Require the release Environment');
  check(environment.deployment_branch_policy?.custom_branch_policies === true && environment.deployment_branch_policy.protected_branches === false, 'Require custom main-only deployment policy');
  check(policies?.length === 1 && policies[0].name === 'main' && policies[0].type === 'branch', 'Only the main branch may deploy; tags and other rules must be absent');
  check(environment.protection_rules?.some(rule => rule.type === 'required_reviewers' && rule.reviewers?.some(item => item.type === 'User' && item.reviewer?.id === reviewerId)), 'Require the selected maintainer reviewer');
  check(!requireSecret || secretPresent === true, 'Release Environment signing Secret metadata is missing');
}

export async function configureReleaseEnvironment({ root, apply = false, reviewer = 'Parsa-Emami', keyPath = join(homedir(), '.local/share/attestara/release-signing/v1/private.pem'), run = runGh }) {
  check(/^[a-z\d](?:[a-z\d-]{0,37}[a-z\d])?$/i.test(reviewer), 'Invalid reviewer login');
  const manifest = JSON.parse(await readFile(join(root, 'docs/release/RELEASE_MANIFEST.json'))); validateManifest(manifest);
  let key;
  try {
    // A missing or wrong original key prevents every remote mutation.
    if (apply) key = await readSigningKey({ root, keyPath, manifest });
    const api = (path, method = 'GET', body) => {
      const output = run(['api', '--hostname', 'github.com', '--method', method, path, ...(body ? ['--input', '-'] : [])], body ? { input: JSON.stringify(body) } : {});
      return output.trim() ? JSON.parse(output) : null;
    };
    const complete = (value, field) => {
      check(Array.isArray(value?.[field]) && Number.isSafeInteger(value.total_count) && value.total_count === value[field].length, 'Missing or truncated GitHub configuration response');
      return value[field];
    };
    const actor = api('/user'), repo = api(prefix);
    check(actor.login?.toLowerCase() === 'parsa-emami' && repo.permissions?.admin === true, 'Run owner setup as the repository owner with administration access');
    const user = api('/users/' + reviewer);
    check(Number.isSafeInteger(user.id) && user.id > 0, 'Reviewer identity is unavailable');
    const permission = api(prefix + '/collaborators/' + reviewer + '/permission');
    check(['read', 'triage', 'write', 'maintain', 'admin'].includes(permission.permission), 'Reviewer must have repository read access or higher');
    const environment = () => complete(api(prefix + '/environments?per_page=100'), 'environments').find(item => item.name === 'release');
    const policies = () => complete(api(environmentPath + '/deployment-branch-policies?per_page=100'), 'branch_policies');
    const existing = environment();
    if (apply) {
      const previous = existing?.protection_rules?.find(rule => rule.type === 'required_reviewers');
      const reviewers = (previous?.reviewers ?? []).map(item => ({ type: item.type, id: item.reviewer.id }));
      if (!reviewers.some(item => item.type === 'User' && item.id === user.id)) reviewers.push({ type: 'User', id: user.id });
      check(reviewers.length <= 6, 'Existing reviewer list is full; preserve it for an owner decision');
      api(environmentPath, 'PUT', { wait_timer: existing?.protection_rules?.find(rule => rule.type === 'wait_timer')?.wait_timer ?? 0, prevent_self_review: previous?.prevent_self_review ?? false, reviewers, deployment_branch_policy: { protected_branches: false, custom_branch_policies: true } });
      const rules = policies();
      const main = rules.find(rule => rule.name === 'main' && rule.type === 'branch');
      if (!main) api(environmentPath + '/deployment-branch-policies', 'POST', { name: 'main', type: 'branch' });
      for (const rule of rules) {
        check(Number.isSafeInteger(rule.id), 'Invalid branch policy identity');
        if (rule !== main) api(environmentPath + '/deployment-branch-policies/' + rule.id, 'DELETE');
      }
      assertReleaseEnvironment({ environment: environment(), policies: policies(), reviewerId: user.id, requireSecret: false });
      run(['secret', 'set', secretName, '--env', 'release', '--repo', manifest.repository], { input: key });
    }
    const current = environment();
    check(current, 'Release Environment is missing; owner configuration is required');
    const rules = policies();
    const secretPresent = complete(api(environmentPath + '/secrets?per_page=100'), 'secrets').some(item => item.name === secretName);
    assertReleaseEnvironment({ environment: current, policies: rules, reviewerId: user.id, secretPresent });
    return { verified: true, applied: apply, repository: manifest.repository, environment: 'release', branches: ['main'], reviewer, prevent_self_review: current.protection_rules.find(rule => rule.type === 'required_reviewers').prevent_self_review, secret_metadata_present: secretPresent, secret_value_verified: false, signer_fingerprint: manifest.fingerprint, publication_performed: false };
  } finally { key?.fill(0); }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const { values } = parseArgs({ options: { apply: { type: 'boolean', default: false }, reviewer: { type: 'string', default: 'Parsa-Emami' } } });
    const result = await configureReleaseEnvironment({ root: fileURLToPath(new URL('../', import.meta.url)), ...values });
    console.log(JSON.stringify(result, null, 2));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
