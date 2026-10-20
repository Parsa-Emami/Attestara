import { createHash, createPrivateKey, createPublicKey, generateKeyPairSync, sign } from 'node:crypto';
import { readFile, writeFile, mkdir, mkdtemp, rm, lstat } from 'node:fs/promises';
import { createWriteStream, createReadStream } from 'node:fs';
import { Readable, Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { tmpdir } from 'node:os';
import { join, resolve, relative, isAbsolute } from 'node:path';
import { parseArgs } from 'node:util';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { verifyRelease } from './verify-release.mjs';

const repository = 'Parsa-Emami/Attestara';
const prefix = '/repos/' + repository;
const workflows = ['contracts.yml', 'plan.yml', 'runtime.yml', 'web.yml', 'release-gate.yml'];
const check = (condition, message) => { if (!condition) throw new Error(message); };
export const digest = bytes => 'sha256:' + createHash('sha256').update(bytes).digest('hex');
async function fileDigest(path) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return 'sha256:' + hash.digest('hex');
}
export const releaseFiles = commit => ['release.json', 'sbom.cdx.json', `attestara-${commit}-linux-x64.tar.gz`, `attestara-${commit}-source.tar.gz`, 'SHA256SUMS.txt', 'SHA256SUMS.sig.json'];
const validCommit = commit => check(/^[a-f0-9]{40}$/.test(commit ?? ''), 'Require a full source commit');

export function validateManifest(manifest) {
  check(manifest.schema_version === 1 && manifest.repository === repository && manifest.version === '1.0.0' && manifest.tag === 'v1.0.0' && manifest.profile === 'local-cli-snapshots' && manifest.platform === 'linux-x64', 'Unsupported release manifest');
  check(manifest.signer_id === 'attestara-release-v1' && /^[a-f0-9]{64}$/.test(manifest.public_key_hex ?? ''), 'Invalid release signer key');
  check(manifest.fingerprint === digest(Buffer.from(manifest.public_key_hex, 'hex')), 'Release key fingerprint mismatch');
}

export function assertCandidate({ commit, main, runs, report, tagExists, releaseExists }) {
  validCommit(commit);
  check(main === commit, 'Candidate must be the exact current main commit');
  check(!tagExists && !releaseExists, 'Release/tag already exists; never overwrite it');
  check(report?.passed === true && report.planning_template === false && report.task === 'C39' && report.profile === 'local-cli-snapshots' && report.source?.commit === commit && report.results?.source_unchanged === true && report.blockers?.length === 0 && report.commands?.length >= 28 && report.commands.every(c => c.exit_code === 0 && !c.signal && !c.timed_out && !c.interrupted), 'Fresh complete gate report required for this source');
  for (const name of workflows) {
    const matching = runs.filter(run => run.path === '.github/workflows/' + name && run.head_sha === commit && run.head_branch === 'main' && ['push', 'workflow_dispatch'].includes(run.event)).sort((a, b) => b.id - a.id);
    check(matching[0]?.status === 'completed' && matching[0]?.conclusion === 'success', 'Latest candidate workflow must succeed: ' + name);
  }
}

export function findPreparedRun(commit, runs, currentRunId) {
  validCommit(commit);
  const prior = runs.filter(run => run.path === '.github/workflows/release.yml' && run.head_sha === commit && run.head_branch === 'main' && run.event === 'workflow_dispatch' && run.display_title === `release ${commit} / publish=false` && Number.isSafeInteger(run.id) && String(run.id) !== String(currentRunId)).sort((a, b) => b.id - a.id)[0];
  check(prior?.status === 'completed' && prior.conclusion === 'success', 'Latest publish=false preparation must succeed on this exact source before publication');
  return prior;
}

// Restrict authenticated requests to GitHub's API and this repository's upload endpoint.
export async function github(path, { method = 'GET', body, upload = false } = {}) {
  const url = upload ? new URL(path) : new URL('https://api.github.com' + path);
  check(upload ? url.origin === 'https://uploads.github.com' && url.pathname.startsWith(prefix + '/releases/') : path.startsWith(prefix + '/'), 'Unsafe GitHub API path');
  const response = await fetch(url, { method, redirect: 'error', signal: AbortSignal.timeout(120000), headers: { Accept: 'application/vnd.github+json', ...(process.env.GITHUB_TOKEN ? { Authorization: 'Bearer ' + process.env.GITHUB_TOKEN } : {}), 'Content-Type': upload ? 'application/octet-stream' : 'application/json' }, body: body === undefined ? undefined : upload ? body : JSON.stringify(body) });
  if (method === 'GET' && response.status === 404) return null;
  check(response.ok, 'GitHub API request failed: ' + response.status);
  return response.json();
}

async function candidateState(commit, report, manifest, request, requirePrepared = false) {
  validateManifest(manifest); validCommit(commit);
  if (process.env.GITHUB_RUN_ID) check(report?.run?.id === process.env.GITHUB_RUN_ID && report.run.kind === 'github-actions', 'Gate must belong to this exact workflow run');
  const [main, runs, tag, release, releases] = await Promise.all([
    request(prefix + '/branches/main'), request(prefix + '/actions/runs?head_sha=' + commit + '&per_page=100'),
    request(prefix + '/git/ref/tags/' + manifest.tag), request(prefix + '/releases/tags/' + manifest.tag), request(prefix + '/releases?per_page=100'),
  ]);
  check(Array.isArray(runs?.workflow_runs) && runs.total_count <= 100, 'Missing or truncated workflow history');
  check(Array.isArray(releases) && releases.length < 100, 'Missing or truncated draft release history');
  assertCandidate({ commit, main: main?.commit?.sha, runs: runs.workflow_runs, report, tagExists: !!tag, releaseExists: !!release || releases.some(r => r.tag_name === manifest.tag) });
  let preparation;
  if (requirePrepared) {
    const prior = findPreparedRun(commit, runs.workflow_runs, process.env.GITHUB_RUN_ID);
    const [jobs, artifacts] = await Promise.all([request(prefix + '/actions/runs/' + prior.id + '/jobs?per_page=100'), request(prefix + '/actions/runs/' + prior.id + '/artifacts?per_page=100')]);
    check(jobs?.total_count === 3 && jobs.jobs?.length === 3 && jobs.jobs.find(job => job.name === 'prepare')?.conclusion === 'success' && ['publish', 'installed'].every(name => jobs.jobs.find(job => job.name === name)?.conclusion === 'skipped'), 'Preparation must finish successfully without publication or installed execution');
    check(Array.isArray(artifacts?.artifacts) && artifacts.total_count === artifacts.artifacts.length, 'Missing or truncated preparation artifacts');
    const artifact = artifacts.artifacts.find(item => item.name === `release-candidate-${commit}-${prior.id}` && item.expired === false && Number.isSafeInteger(item.id));
    check(artifact, 'Retained unsigned preparation candidate is required for review');
    preparation = { run_id: prior.id, url: prior.html_url, artifact_id: artifact.id };
  }
  return { commit, ...(preparation ? { preparation } : {}), workflows: runs.workflow_runs.filter(r => workflows.some(name => r.path === '.github/workflows/' + name)).map(r => ({ id: r.id, path: r.path, conclusion: r.conclusion, url: r.html_url })) };
}

export async function assessRelease({ directory, commit, manifest, signature = true }) {
  validateManifest(manifest); validCommit(commit);
  const work = await mkdtemp(join(tmpdir(), 'attestara-release-trust-'));
  try {
    const trust = join(work, 'trust.json');
    await writeFile(trust, JSON.stringify({ schema_version: '1', trusted_signers: { [manifest.signer_id]: manifest.public_key_hex } }), { mode: 0o600 });
    return await verifyRelease({ directory, signature: signature ? join(directory, 'SHA256SUMS.sig.json') : undefined, trust, expectedSourceCommit: commit, freshness: 'fresh' });
  } finally { await rm(work, { recursive: true, force: true }); }
}

export async function signChecksums({ directory, commit, privateKey, manifest }) {
  const before = await assessRelease({ directory, commit, manifest, signature: false });
  check(before.assessment.file_integrity === 'verified' && before.assessment.expected_source_match, 'Cannot sign an invalid package');
  let key;
  try { key = privateKey?.type === 'private' ? privateKey : createPrivateKey(privateKey); }
  catch { throw new Error('Release private key is unavailable or invalid'); }
  check(key.asymmetricKeyType === 'ed25519', 'Require an Ed25519 release key');
  const publicKey = Buffer.from(createPublicKey(key).export({ format: 'jwk' }).x, 'base64url').toString('hex');
  check(publicKey === manifest.public_key_hex, 'Private key differs from the independently pinned release key');
  const sums = await readFile(join(directory, 'SHA256SUMS.txt'));
  const detached = { schema_version: '1', signer_id: manifest.signer_id, public_key_hex: publicKey, signature_hex: sign(null, sums, key).toString('hex') };
  await writeFile(join(directory, 'SHA256SUMS.sig.json'), JSON.stringify(detached, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  const result = await assessRelease({ directory, commit, manifest });
  check(result.exitCode === 0, 'Signed release verification failed');
  return result.assessment;
}

export async function publishRelease({ directory, commit, report, manifest, authorized, request = github }) {
  check(authorized === true, 'Explicit publication authorization required');
  const verified = await assessRelease({ directory, commit, manifest });
  check(verified.exitCode === 0, 'Trusted signed artifacts required before publication');
  const candidate = await candidateState(commit, report, manifest, request, true);
  const draft = await request(prefix + '/releases', { method: 'POST', body: { tag_name: manifest.tag, target_commitish: commit, name: 'Attestara ' + manifest.version, draft: true, prerelease: false, make_latest: 'false', body: `Linux x86_64 local-cli-snapshots. Source: ${commit}. Verify SHA256SUMS.sig.json with the independently established maintainer key before extracting. C40 installation acceptance is recorded separately after publication.` } });
  check(Number.isSafeInteger(draft.id) && draft.upload_url === `https://uploads.github.com${prefix}/releases/${draft.id}/assets{?name,label}`, 'Unexpected draft release response');
  const uploaded = [];
  for (const name of releaseFiles(commit)) {
    const bytes = await readFile(join(directory, name));
    const asset = await request(draft.upload_url.replace('{?name,label}', '?name=' + encodeURIComponent(name)), { method: 'POST', body: bytes, upload: true });
    check(asset.digest === digest(bytes) && asset.size === bytes.length && asset.name === name && asset.state === 'uploaded', 'Uploaded asset differs from reviewed bytes: ' + name);
    uploaded.push({ name, digest: asset.digest, size: asset.size });
  }
  const stored = await request(prefix + '/releases/' + draft.id);
  check(stored?.draft === true && stored.assets?.length === uploaded.length && uploaded.every(a => stored.assets.some(b => a.name === b.name && a.digest === b.digest && a.size === b.size && b.state === 'uploaded')), 'Incomplete draft release');
  const createdTag = await request(prefix + '/git/refs', { method: 'POST', body: { ref: 'refs/tags/' + manifest.tag, sha: commit } });
  check(createdTag?.object?.type === 'commit' && createdTag.object.sha === commit, 'Release tag differs from exact reviewed source');
  const published = await request(prefix + '/releases/' + draft.id, { method: 'PATCH', body: { draft: false, make_latest: 'true' } });
  check(published.draft === false && published.tag_name === manifest.tag, 'Release publication did not complete');
  return { published: true, candidate, release_id: draft.id, url: published.html_url, assets: uploaded, assessment: verified.assessment };
}

export async function downloadFile(url, destination) {
  const response = await fetch(url, { signal: AbortSignal.timeout(300000), redirect: 'follow' });
  check(response.ok, 'Published asset download failed: ' + response.status);
  let size = 0;
  const limit = new Transform({ transform(chunk, encoding, callback) { size += chunk.length; callback(size > 1024 * 1024 * 1024 ? new Error('Published asset exceeds byte limit') : null, chunk); } });
  await pipeline(Readable.fromWeb(response.body), limit, createWriteStream(destination, { flags: 'wx', mode: 0o600 }));
}

export async function downloadRelease({ directory, commit, manifest, request = github, fetchFile = downloadFile }) {
  validateManifest(manifest); validCommit(commit);
  const release = await request(prefix + '/releases/tags/' + manifest.tag);
  check(release && release.draft === false && release.prerelease === false && release.tag_name === manifest.tag && Number.isSafeInteger(release.id), 'Require the actual public stable release');
  const ref = await request(prefix + '/git/ref/tags/' + manifest.tag);
  check(ref?.object?.type === 'commit' && ref.object.sha === commit, 'Published tag differs from independently expected source');
  const names = releaseFiles(commit);
  check(release.assets?.length === names.length && new Set(release.assets.map(a => a.name)).size === names.length, 'Missing, duplicate or extra published assets');
  for (const name of names) {
    const asset = release.assets.find(asset => asset.name === name);
    check(asset?.state === 'uploaded' && Number.isSafeInteger(asset.size) && asset.size > 0 && asset.size <= 1024 * 1024 * 1024 && asset.browser_download_url === `https://github.com/${repository}/releases/download/${manifest.tag}/${name}`, 'Unsafe or incomplete published asset');
  }
  await mkdir(directory, { recursive: false, mode: 0o700 });
  for (const name of names) {
    const asset = release.assets.find(asset => asset.name === name);
    const destination = join(directory, name);
    await fetchFile(asset.browser_download_url, destination);
    check((await lstat(destination)).isFile() && (await lstat(destination)).size === asset.size && await fileDigest(destination) === asset.digest, 'Published asset digest/size differs');
  }
  const verified = await assessRelease({ directory, commit, manifest });
  check(verified.exitCode === 0, 'Downloaded release is not independently trusted');
  return { release_id: release.id, tag: manifest.tag, url: release.html_url, source_commit: commit, assets: release.assets.map(a => ({ id: a.id, name: a.name, digest: a.digest, size: a.size })), assessment: verified.assessment };
}

export async function createReleaseKey(directory, root) {
  check(isAbsolute(directory) && relative(root, directory).startsWith('../'), 'Private key directory must be outside the repository');
  await mkdir(directory, { mode: 0o700 });
  const keys = generateKeyPairSync('ed25519');
  const publicKey = Buffer.from(keys.publicKey.export({ format: 'jwk' }).x, 'base64url').toString('hex');
  await writeFile(join(directory, 'private.pem'), keys.privateKey.export({ type: 'pkcs8', format: 'pem' }), { flag: 'wx', mode: 0o600 });
  const publicInfo = { signer_id: 'attestara-release-v1', public_key_hex: publicKey, fingerprint: digest(Buffer.from(publicKey, 'hex')) };
  await writeFile(join(directory, 'public.json'), JSON.stringify(publicInfo, null, 2) + '\n', { flag: 'wx', mode: 0o600 });
  return publicInfo;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const root = fileURLToPath(new URL('../', import.meta.url));
    const action = process.argv[2];
    const { values } = parseArgs({ args: process.argv.slice(3), options: { 'source-commit': { type: 'string' }, directory: { type: 'string' }, report: { type: 'string' }, output: { type: 'string' }, 'key-directory': { type: 'string' }, publish: { type: 'boolean', default: false } } });
    let result;
    if (action === 'init-key') result = await createReleaseKey(resolve(values['key-directory']), root);
    else {
      const manifest = JSON.parse(await readFile(join(root, 'docs/release/RELEASE_MANIFEST.json')));
      const commit = values['source-commit']; validCommit(commit);
      const report = values.report ? JSON.parse(await readFile(values.report)) : undefined;
      const directory = values.directory ? resolve(values.directory) : undefined;
      if (action === 'check') result = await candidateState(commit, report, manifest, github);
      else if (action === 'sign') result = await signChecksums({ directory, commit, manifest, privateKey: process.env.ATTESTARA_RELEASE_PRIVATE_KEY });
      else if (action === 'publish') {
        check(process.env.GITHUB_ACTIONS === 'true' && process.env.GITHUB_EVENT_NAME === 'workflow_dispatch' && process.env.GITHUB_REF === 'refs/heads/main' && process.env.GITHUB_SHA === commit && process.env.GITHUB_REPOSITORY === repository, 'Publish only through the manually authorized main workflow');
        result = await publishRelease({ directory, commit, manifest, report, authorized: values.publish });
      }
      else throw new Error('Usage: release-publication.mjs init-key|check|sign|publish');
    }
    const bytes = JSON.stringify(result, null, 2) + '\n';
    if (values.output) await writeFile(values.output, bytes, { flag: 'wx', mode: 0o600 });
    console.log(bytes);
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
