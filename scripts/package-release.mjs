import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFile, writeFile, mkdir, mkdtemp, readdir, lstat, cp, rm, access, open } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { sourceSnapshot, copySource } from './package-source.mjs';
import { verifyResources } from './verify-resources.mjs';

export const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
export function safeName(name) {
  if (typeof name !== 'string' || !name || /[\\\x00-\x1f\x7f]/.test(name) || name.split('/').some(part => !part || part === '.' || part === '..' || part.startsWith('-')) || name.startsWith('/')) throw new Error('Unsafe release path');
  return name;
}
export async function inventory(root, prefix = '') {
  const files = [];
  for (const name of (await readdir(join(root, prefix))).sort()) {
    const path = safeName(prefix + name), info = await lstat(join(root, path));
    if (info.isDirectory()) files.push(...await inventory(root, path + '/'));
    else if (info.isFile()) files.push({ path, sha256: sha256(await readFile(join(root, path))), mode: info.mode & 0o111 ? 0o755 : 0o644 });
    else throw new Error('Release entries must be regular files');
  }
  return files;
}
export async function verifyInventory(root, files) {
  if (!(await lstat(root)).isDirectory()) throw new Error('Release root must be a real directory');
  if (!Array.isArray(files) || !files.length) throw new Error('Empty release inventory');
  const seen = new Set(), parents = new Set();
  for (const file of files) {
    safeName(file.path);
    if (seen.has(file.path) || !/^[a-f0-9]{64}$/.test(file.sha256) || ![0o644, 0o755].includes(file.mode)) throw new Error('Invalid release inventory');
    seen.add(file.path);
    const parts = file.path.split('/');
    for (let i = 1; i < parts.length; i++) {
      const parent = parts.slice(0, i).join('/');
      if (!parents.has(parent)) {
        if (!(await lstat(join(root, parent))).isDirectory()) throw new Error('Release parent must be a real directory');
        parents.add(parent);
      }
    }
    const info = await lstat(join(root, file.path));
    if (!info.isFile() || (info.mode & 0o111 ? 0o755 : 0o644) !== file.mode || sha256(await readFile(join(root, file.path))) !== file.sha256) throw new Error('Release checksum mismatch: ' + file.path);
  }
}
const run = (command, args, cwd, options = {}) => execFileSync(command, args, { cwd, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 600000, ...options });
export async function publish(stage, destination) {
  try { await lstat(destination); throw new Error('Release destination exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  // GNU mv uses no-replace publication; inspect staging because -n also succeeds on a skipped move.
  run('mv', ['--no-clobber', '--no-target-directory', stage, destination]);
  try { await access(stage); } catch (error) { if (error.code === 'ENOENT') return; throw error; }
  throw new Error('Release destination exists');
}
export async function archive(root, files, output, epoch) {
  const list = output + '.list';
  await writeFile(list, files.map(file => safeName(file.path) + '\0').join(''));
  try {
    run('tar', ['--create', '--gzip', '--file', output, '--format=gnu', '--sort=name', '--owner=0', '--group=0', '--numeric-owner', '--mtime=@' + epoch, '--mode=u+rwX,go+rX,go-w', '--null', '--verbatim-files-from', '--no-recursion', '--files-from', list], root, { env: { ...process.env, LC_ALL: 'C', TZ: 'UTC', GZIP: '', TAR_OPTIONS: '' } });
  } finally { await rm(list, { force: true }); }
}
async function json(root, path) { return JSON.parse(await readFile(join(root, path), 'utf8')); }
async function save(root, name, value) { await writeFile(join(root, name), JSON.stringify(value, null, 2) + '\n'); }
async function copy(root, destination, path) {
  safeName(path); await mkdir(join(destination, path, '..'), { recursive: true });
  await cp(join(root, path), join(destination, path), { recursive: true, dereference: false });
}

async function billOfMaterials(build, runtime, source, engine, lock) {
  const npm = await json(build, 'package-lock.json');
  const components = [], licenses = [];
  for (const [path, pkg] of Object.entries(npm.packages).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)) {
    if (!path) continue;
    safeName(path);
    if (!pkg.license || !pkg.version || !pkg.integrity || !pkg.resolved?.startsWith('https://registry.npmjs.org/')) throw new Error('Unpinned or unlicensed npm dependency: ' + path);
    const name = path.slice(path.lastIndexOf('node_modules/') + 13);
    components.push({ type: 'library', name, version: pkg.version, 'bom-ref': path, scope: pkg.dev ? 'optional' : 'required', licenses: [{ expression: pkg.license }], properties: [{ name: 'attestara:distribution', value: pkg.dev ? 'build-only' : 'web-bundle' }, { name: 'attestara:npm-integrity', value: pkg.integrity }], externalReferences: [{ type: 'distribution', url: pkg.resolved }] });
    // Only React, scheduler and Vite's runtime helper contribute upstream JS to the web bundle.
    if (pkg.dev && name !== 'vite') continue;
    const directory = join(build, path);
    const texts = (await readdir(directory)).filter(file => /^(licen[cs]e|notice|copying)(\.|$)/i.test(file)).sort();
    if (!texts.length) throw new Error('Missing distributed license text: ' + name);
    for (const text of texts) {
      const output = 'licenses/npm/' + name.replaceAll('/', '__') + '/' + text;
      await mkdir(join(runtime, output, '..'), { recursive: true });
      await cp(join(directory, text), join(runtime, output)); licenses.push(output);
    }
  }
  const buildInfo = run('go', ['version', '-m', engine], build);
  const modules = [...buildInfo.matchAll(/^\s*dep\s+(\S+)\s+(\S+)\s+(h1:\S+)/gm)];
  const goLicenses = {
    'github.com/santhosh-tekuri/jsonschema/v6': ['Apache-2.0', 'jsonschema-v6-LICENSE.txt'],
    'golang.org/x/text': ['BSD-3-Clause', 'golang-x-text-LICENSE.txt', 'golang-x-text-PATENTS.txt'],
  };
  for (const [, name, version, sum] of modules) {
    const license = goLicenses[name];
    if (!license) throw new Error('Unknown distributed Go dependency license: ' + name);
    if (!(await readFile(join(build, 'go.sum'), 'utf8')).includes(name + ' ' + version + ' ' + sum)) throw new Error('Go binary differs from locked modules');
    components.push({ type: 'library', name, version, 'bom-ref': name, scope: 'required', licenses: [{ expression: license[0] }], properties: [{ name: 'attestara:go-sum', value: sum }, { name: 'attestara:distribution', value: 'engine' }] });
    for (const text of license.slice(1)) licenses.push('docs/engineering/resources/' + text);
  }
  if (modules.length !== Object.keys(goLicenses).length) throw new Error('Incomplete linked module inventory');
  for (const id of ['go', 'cyclonedx']) {
    const resource = lock.resources.find(item => item.id === id);
    licenses.push(resource.license.path);
    components.push({ type: id === 'go' ? 'framework' : 'data', name: id, version: resource.version, 'bom-ref': 'resource:' + id, scope: 'required', licenses: [{ expression: resource.license.expression }], properties: [{ name: 'attestara:distribution', value: id === 'go' ? 'linked-runtime' : 'embedded-schemas' }], externalReferences: [{ type: 'vcs', url: resource.source_url }] });
  }
  const bom = { bomFormat: 'CycloneDX', specVersion: '1.7', version: 1, metadata: { component: { type: 'application', name: 'Attestara', version: source.sourceCommit, 'bom-ref': 'attestara', licenses: [{ expression: 'Apache-2.0' }] } }, components, dependencies: [{ ref: 'attestara', dependsOn: components.map(item => item['bom-ref']) }] };
  await save(runtime, 'sbom.cdx.json', bom);
  await save(runtime, 'license-inventory.json', { schemaVersion: 1, distributedLicenseTexts: licenses, buildOnlyDependencies: 'See SBOM properties and package-lock.json; their code is not shipped in the runtime artifact.', externalPrerequisites: ['Node 24.19.0'], excludedProviders: ['OPA', 'Syft', 'Grype'] });
  // Reuse the actual C30 CLI and embedded offline schema validator, with fresh hashed input bytes.
  const probe = await mkdtemp(join(build, 'dist/sbom-check-'));
  const manifest = await json(build, 'tests/fixtures/attestara/C30/manifest.json');
  const bytes = await readFile(join(runtime, 'sbom.cdx.json'));
  await writeFile(join(probe, 'sbom.cdx.json'), bytes);
  for (const name of ['subject.txt', 'rule.json', 'evidence.json']) await cp(join(build, 'tests/fixtures/attestara/C30', name), join(probe, name));
  manifest.files = manifest.files.filter(file => file.path !== 'sbom.cdx.json');
  manifest.files.push({ path: 'sbom.cdx.json', digest: 'sha256:' + sha256(bytes), size_bytes: bytes.length, media_type: 'application/vnd.cyclonedx+json' });
  await save(probe, 'manifest.json', manifest);
  run(engine, ['evaluate', '--input', join(probe, 'manifest.json'), '--files', probe, '--policy', join(probe, 'rule.json'), '--evidence', join(probe, 'evidence.json'), '--freshness', 'fresh'], build);
}

export async function packageRelease(root = fileURLToPath(new URL('../', import.meta.url))) {
  root = resolve(root);
  const source = await sourceSnapshot(root);
  const lock = await json(root, 'docs/engineering/toolchain-lock.json');
  if (process.platform !== 'linux' || process.arch !== 'x64' || process.version !== 'v' + lock.runtime.node) throw new Error('Locked Linux x64 Node runtime required');
  const npm = process.env.npm_execpath;
  if (!npm || run(process.execPath, [npm, '--version'], root).trim() !== lock.runtime.npm) throw new Error('Run through locked npm ' + lock.runtime.npm);
  if (run('go', ['env', 'GOVERSION'], root, { env: { ...process.env, GOTOOLCHAIN: 'local' } }).trim() !== 'go' + lock.resources.find(item => item.id === 'go').version) throw new Error('Locked Go toolchain required');
  await verifyResources({ root });
  const parent = join(root, 'dist/packages'); await mkdir(parent, { recursive: true });
  const destination = join(parent, source.sourceCommit);
  try { await lstat(destination); throw new Error('Release destination exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
  const stage = await mkdtemp(join(parent, '.c31-'));
  try {
    const build = join(stage, 'source'), runtime = join(stage, 'runtime'), output = join(stage, 'output');
    await mkdir(build); await mkdir(runtime); await mkdir(output);
    await copySource(root, build, source);
    const env = { ...process.env, GOTOOLCHAIN: 'local', CGO_ENABLED: '0', GOFLAGS: '', GOOS: 'linux', GOARCH: 'amd64', GOAMD64: 'v1', GOWORK: 'off', NODE_ENV: 'development' };
    run(process.execPath, [npm, 'ci', '--ignore-scripts', '--no-audit', '--no-fund'], build, { env, stdio: 'inherit' });
    run(process.execPath, [npm, 'run', 'build'], build, { env: { ...env, NODE_ENV: 'production' }, stdio: 'inherit' });
    await mkdir(join(build, 'dist/engine'), { recursive: true });
    const engine = join(build, 'dist/engine/attestara');
    run('go', ['build', '-mod=readonly', '-trimpath', '-buildvcs=false', '-o', engine, './cmd/attestara'], build, { env, stdio: 'inherit' });
    // Build hooks may not silently change the archived source inputs.
    await verifyInventory(build, source.files);
    for (const path of ['dist/engine', 'dist/web', 'scripts/start-local.mjs', 'scripts/serve-web.mjs', 'scripts/verify-release.mjs', 'apps/web/src/lib/runtime-config.mjs', 'apps/web/src/lib/engine-health.mjs', 'ops/web', 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'docs/engineering/resources', 'docs/engineering/C30_LOCAL_WORKFLOW.md', 'docs/engineering/WEB_RUNBOOK.md', 'docs/release/ARTIFACTS.md', 'tests/fixtures/attestara/C30', 'tests/scenarios/clone-to-value.json']) await copy(build, runtime, path);
    await cp(join(build, 'docs/release/ARTIFACTS.md'), join(runtime, 'README.md'));
    await billOfMaterials(build, runtime, source, engine, lock);
    const metadata = {
      schemaVersion: 1, sourceCommit: source.sourceCommit,
      sourceIdentity: 'sha256:' + sha256(Buffer.from(JSON.stringify(source.files))),
      sourceEpoch: source.epoch, scope: 'local-cli-snapshots', platform: 'linux-x64',
      tools: { node: lock.runtime.node, npm: lock.runtime.npm, go: lock.resources.find(item => item.id === 'go').version },
      build: { cgo: false, trimpath: true, buildvcs: false },
      prerequisites: ['Node 24.19.0'], providersBundled: [],
      packagingTools: {
        tar: run('tar', ['--version'], build).split('\n')[0],
        gzip: run('gzip', ['--version'], build).split('\n')[0],
        coreutils: run('mv', ['--version'], build).split('\n')[0],
      },
      authenticity: 'unsigned payload; authenticate SHA256SUMS.txt with a separately trusted detached release signature',
      inputs: Object.fromEntries(await Promise.all(
        ['package-lock.json', 'go.mod', 'go.sum', 'docs/engineering/toolchain-lock.json']
          .map(async path => [path, sha256(await readFile(join(build, path)))]),
      )),
    };
    await save(runtime, 'release.json', metadata);
    const runtimeFiles = await inventory(runtime);
    await writeFile(join(runtime, 'SHA256SUMS.txt'), runtimeFiles.map(file => file.sha256 + '  ' + file.path).join('\n') + '\n');
    await verifyInventory(runtime, runtimeFiles);
    const stem = 'attestara-' + source.sourceCommit;
    await archive(runtime, await inventory(runtime), join(output, stem + '-linux-x64.tar.gz'), source.epoch);
    await archive(build, [...source.files, { path: 'SOURCE_MANIFEST.json' }], join(output, stem + '-source.tar.gz'), source.epoch);
    await save(output, 'release.json', metadata);
    await cp(join(runtime, 'sbom.cdx.json'), join(output, 'sbom.cdx.json'));
    await writeFile(join(output, 'SHA256SUMS.txt'), (await inventory(output)).map(file => file.sha256 + '  ' + file.path).join('\n') + '\n');
    // Flush completed bytes before the final no-replace directory publication.
    for (const file of await inventory(output)) { const handle = await open(join(output, file.path), 'r'); try { await handle.sync(); } finally { await handle.close(); } }
    const handle = await open(output, 'r'); try { await handle.sync(); } finally { await handle.close(); }
    await publish(output, destination);
    const parentHandle = await open(parent, 'r'); try { await parentHandle.sync(); } finally { await parentHandle.close(); }
    console.log('Complete unsigned local-profile artifacts: ' + destination);
    return destination;
  } finally { await rm(stage, { recursive: true, force: true }); }
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  packageRelease().catch(error => { console.error(error.message); process.exitCode = 1; });
}
