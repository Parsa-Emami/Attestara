import { readFile, realpath, stat } from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import { createHash } from 'node:crypto';
import { join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = fileURLToPath(new URL('../', import.meta.url));
const check = (condition, message) => { if (!condition) throw new Error(message); };
const present = value => typeof value === 'string' && value.trim().length > 0;
const inside = (parent, child) => child.startsWith(parent + sep);
const sources = {
  node: 'https://nodejs.org/dist/v', npm: 'https://github.com/npm/cli/releases/tag/v',
  go: 'https://go.dev/dl/#go', opa: 'https://github.com/open-policy-agent/opa/releases/tag/v',
  syft: 'https://github.com/anchore/syft/releases/tag/v', grype: 'https://github.com/anchore/grype/releases/tag/v',
  cyclonedx: 'https://github.com/CycloneDX/specification/tree/', spdx: 'https://github.com/spdx/spdx-spec/tree/',
};
const licenses = new Set(['Apache-2.0', 'MIT', 'BSD-3-Clause', 'ISC', 'MPL-2.0', 'Artistic-2.0', 'Community-Spec-1.0 AND CC-BY-3.0']);
const officialHosts = new Set(['go.dev', 'nodejs.org', 'registry.npmjs.org', 'github.com', 'api.github.com', 'raw.githubusercontent.com', 'codeload.github.com']);
function official(url) {
  try {
    const parsed = new URL(url);
    check(parsed.protocol === 'https:' && !parsed.port && !parsed.username && !parsed.password && officialHosts.has(parsed.hostname) && !/(?:^|\/)(?:latest|main|master|HEAD)(?:\/|$)/.test(parsed.pathname), 'Missing official source or mutable URL');
  } catch { throw new Error('Missing official source or mutable URL'); }
}
async function json(path) {
  check((await stat(path)).size <= 4 * 1024 * 1024, 'Resource metadata exceeds limit');
  return JSON.parse(await readFile(path, 'utf8'));
}
export async function verifyArtifact(path, sha256) {
  check(/^[a-f0-9]{64}$/.test(sha256), 'Invalid SHA-256 checksum');
  check((await stat(path)).isFile(), 'Artifact is not a regular file');
  const digest = createHash('sha256');
  for await (const bytes of createReadStream(path)) digest.update(bytes);
  check(digest.digest('hex') === sha256, `Artifact checksum mismatch: ${path}`);
}

export async function verifyResources({ root = projectRoot, artifacts, platform = `${process.platform}-${process.arch}` } = {}) {
  root = await realpath(root);
  const lock = await json(join(root, 'docs/engineering/toolchain-lock.json'));
  const pkg = await json(join(root, 'package.json'));
  check(lock.schema_version === 1 && lock.profile === 'linux-x64' && platform === lock.profile, 'Unsupported resource version or mandatory platform');
  check(Array.isArray(lock.resources) && lock.resources.length === Object.keys(sources).length, 'Missing or duplicate required resource');
  const ids = new Set();
  async function snapshot(ref) {
    check(ref && /^docs\/engineering\/resources\/[a-zA-Z0-9.-]+$/.test(ref.path), 'Unsafe snapshot path');
    const path = await realpath(join(root, ref.path));
    check(inside(root, path) && inside(await realpath(join(root, 'docs/engineering/resources')), path), 'Unsafe snapshot path');
    official(ref.url);
    await verifyArtifact(path, ref.sha256);
    return path;
  }
  const artifactRoot = artifacts ? await realpath(artifacts) : undefined;
  const verified = [], optionalAbsent = [];
  for (const resource of lock.resources) {
    const { id, version, artifact, integrity, license, source_build: build } = resource;
    check(Object.hasOwn(sources, id) && !ids.has(id), 'Unknown or duplicate resource'); ids.add(id);
    check(/^\d+\.\d+(?:\.\d+)?$/.test(version), `${id}: mutable or missing version`);
    check(resource.required === ['node', 'npm', 'go'].includes(id), `${id}: required/optional prerequisite changed`);
    check(resource.platform === lock.profile, `${id}: unsupported mandatory platform`);
    official(resource.source_url);
    const sourcePin = ['cyclonedx', 'spdx'].includes(id) ? integrity?.source_commit : version;
    check(present(sourcePin) && resource.source_url === sources[id] + sourcePin + (id === 'node' ? '/' : ''), `${id}: official source/version mismatch`);
    check(license && licenses.has(license.expression) && present(resource.redistribution), `${id}: missing or unsupported license/redistribution terms`);
    await snapshot(license);
    check(artifact && /^[a-zA-Z0-9_.-]+$/.test(artifact.filename) && /^[a-f0-9]{64}$/.test(artifact.sha256), `${id}: missing immutable artifact pin`);
    check(present(integrity?.format), `${id}: missing integrity metadata`);
    official(artifact.url);
    const artifactUrls = {
      go: `https://codeload.github.com/golang/go/tar.gz/${integrity.source_commit}`,
      node: `https://nodejs.org/dist/v${version}/${artifact.filename}`,
      npm: `https://registry.npmjs.org/npm/-/npm-${version}.tgz`,
      opa: `https://github.com/open-policy-agent/opa/releases/download/v${version}/${artifact.filename}`,
      syft: `https://github.com/anchore/syft/releases/download/v${version}/${artifact.filename}`,
      grype: `https://github.com/anchore/grype/releases/download/v${version}/${artifact.filename}`,
      cyclonedx: `https://codeload.github.com/CycloneDX/specification/tar.gz/${integrity.source_commit}`,
      spdx: `https://codeload.github.com/spdx/spdx-spec/tar.gz/${integrity.source_commit}`,
    };
    check(artifact.url === artifactUrls[id], `${id}: artifact is not from its official source`);
    if (!['pinned-source', 'git-commit'].includes(integrity?.format)) check(artifact.url.includes(version), `${id}: mutable artifact URL`);
    const metadataPath = await snapshot(integrity);
    if (resource.prebuilt_alternative) {
      const alternative = resource.prebuilt_alternative;
      const releases = await json(await snapshot(alternative.official_metadata));
      const release = releases.find(r => r.version === 'go' + version);
      check(id === 'go' && release?.stable && alternative.url === `https://go.dev/dl/go${version}.linux-amd64.tar.gz` && release.files.some(f => f.filename === `go${version}.linux-amd64.tar.gz` && f.sha256 === alternative.sha256), 'Go alternative published checksum mismatch');
    }
    const metadata = await readFile(metadataPath, 'utf8');
    if (integrity.format === 'sha256sum') {
      check(metadata.split(/\r?\n/).some(line => { const [digest, file] = line.trim().split(/\s+/); return digest === artifact.sha256 && file?.replace(/^\*/, '') === artifact.filename; }), `${id}: published checksum mismatch`);
    } else if (integrity.format === 'npm-dist') {
      const release = JSON.parse(metadata);
      check(release.version === version && release.dist.tarball === artifact.url && release.dist.integrity === artifact.integrity && /^sha512-[A-Za-z0-9+/]+={0,2}$/.test(artifact.integrity), 'npm published integrity mismatch');
    } else {
      check(['pinned-source', 'git-commit'].includes(integrity.format) && /^[a-f0-9]{40}$/.test(integrity.source_commit) && artifact.url.endsWith(integrity.source_commit), `${id}: source commit pin missing`);
      if (integrity.format === 'git-commit') check(JSON.parse(metadata).sha === integrity.source_commit, `${id}: official source commit mismatch`);
    }
    check(build && (present(build.not_applicable) || (Array.isArray(build.commands) && build.commands.length && build.commands.every(present) && present(build.verification))), `${id}: source-build route missing`);
    if (build.recipe) await snapshot(build.recipe);
    const commitMetadata = build.commit_metadata ?? integrity.commit_metadata;
    if (commitMetadata) {
      const commit = await json(await snapshot(commitMetadata));
      check(commit.sha === (build.source_commit ?? integrity.source_commit), `${id}: source-build commit mismatch`);
    }
    if (build.recipe_url) official(build.recipe_url);
    if (build.archive) { official(build.archive.url); check(/^[a-f0-9]{64}$/.test(build.archive.sha256), `${id}: source archive checksum missing`); }
    if (artifactRoot) {
      const path = join(artifactRoot, artifact.filename);
      let real;
      try { real = await realpath(path); } catch (error) {
        if (error.code !== 'ENOENT' || resource.required) throw new Error(`${id}: unavailable ${resource.required ? 'required' : 'optional'} artifact`);
        optionalAbsent.push(id); continue;
      }
      check(inside(artifactRoot, real), 'Unsafe artifact path');
      await verifyArtifact(real, artifact.sha256);
      if (id === 'npm') {
        const sri = 'sha512-' + createHash('sha512').update(await readFile(real)).digest('base64');
        check(sri === artifact.integrity, 'npm artifact SRI checksum mismatch');
      }
      verified.push(id);
    }
  }
  check(Object.keys(sources).every(id => ids.has(id)), 'Missing required resource');
  check(lock.resources.find(r => r.id === 'node').version === lock.runtime.node && lock.resources.find(r => r.id === 'npm').version === lock.runtime.npm && pkg.packageManager === 'npm@' + lock.runtime.npm, 'Web runtime pin mismatch');
  check(lock.resources.find(r => r.id === 'cyclonedx').version === '1.7' && lock.resources.find(r => r.id === 'spdx').version === '3.0', 'Accepted specification versions changed');
  for (const field of ['dependencies', 'devDependencies']) check(JSON.stringify(lock[field]) === JSON.stringify(pkg[field]), 'Web dependency pin mismatch');
  for (const field of ['lockfile', 'notices']) {
    const expected = field === 'lockfile' ? 'package-lock.json' : 'THIRD_PARTY_NOTICES.md';
    check(lock[field]?.path === expected, 'Unsafe inventory path');
    await verifyArtifact(join(root, expected), lock[field].sha256);
  }
  const npmLock = await json(join(root, 'package-lock.json'));
  for (const [path, dependency] of Object.entries(npmLock.packages)) {
    if (!path) continue;
    check(/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(dependency.version) && /^https:\/\/registry\.npmjs\.org\//.test(dependency.resolved) && /^sha512-[A-Za-z0-9+/]+={0,2}$/.test(dependency.integrity) && licenses.has(dependency.license), `Unpinned or unlicensed npm dependency: ${path}`);
  }
  const runtime = await json(join(root, 'ops/runtime-requirements.json'));
  check(runtime.resource_profile === lock.profile && runtime.toolchain_lock === 'docs/engineering/toolchain-lock.json' && runtime.paid_or_cloud_account_required === false && runtime.owned_domain_required === false && JSON.stringify(runtime.required_tools) === JSON.stringify(['node', 'npm', 'go']) && JSON.stringify(runtime.optional_tools) === JSON.stringify(['opa', 'syft', 'grype']), 'Required/optional runtime prerequisite mismatch');
  for (const id of ['container_images', 'policy_rules', 'external_datasets', 'go_module']) check(present(lock.not_applicable?.[id]), `Missing not-applicable reason: ${id}`);
  const fixturePaths = new Set();
  for (const fixture of lock.fixtures ?? []) {
    check(/^tests\/fixtures\/attestara\/C0[12]\/[a-z-]+\.json$/.test(fixture.path) && fixture.license === 'Apache-2.0' && typeof fixture.provenance === 'string' && fixture.provenance.startsWith('repository-authored synthetic'), 'Unsafe fixture path or missing license/provenance');
    const path = await realpath(join(root, fixture.path));
    check(inside(root, path), 'Unsafe fixture path');
    check(!fixturePaths.has(fixture.path), 'Duplicate fixture'); fixturePaths.add(fixture.path);
    await verifyArtifact(path, fixture.sha256);
    const value = await json(path);
    check(value.license === fixture.license && value.provenance === fixture.provenance, 'Fixture license/provenance mismatch');
  }
  check(['C01', 'C02'].every(task => ['positive', 'negative', 'expected'].every(name => fixturePaths.has(`tests/fixtures/attestara/${task}/${name}.json`))), 'Missing C01/C02 licensed fixtures');
  return { resources: ids.size, artifactChecks: verified, optionalAbsent, engineExecutionVerified: false };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const args = process.argv.slice(2);
    check(args.length === 0 || (args.length === 2 && args[0] === '--artifacts'), 'Usage: node scripts/verify-resources.mjs [--artifacts DIRECTORY]');
    console.log(JSON.stringify(await verifyResources({ artifacts: args[1] }), null, 2));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
