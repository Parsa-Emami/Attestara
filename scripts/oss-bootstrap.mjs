import { spawnSync } from 'node:child_process';
import { readFile, access } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { mkdirSync, readFileSync, renameSync, rmSync } from 'node:fs';
import { validateEngineHealth } from '../apps/web/src/lib/engine-health.mjs';
const root = fileURLToPath(new URL('../', import.meta.url));
export const supportedNode = version => { const [major, minor] = version.replace(/^v/, '').split('.').map(Number); return major === 24 && minor >= 19; };
export function setup(args = [], run = spawnSync, version = process.version, directory = root) {
  if (!supportedNode(version)) throw new Error('Use Node 24.19 or a newer Node 24 release.');
  if (args.some(arg => !['--check', '--skip-install'].includes(arg))) throw new Error('Supported setup options: --check, --skip-install');
  const steps = args.includes('--skip-install') ? [['run', 'build']] : [['ci', '--ignore-scripts', '--no-fund'], ['run', 'build']];
  const goVersion = JSON.parse(readFileSync(new URL('../docs/engineering/toolchain-lock.json', import.meta.url))).resources.find(resource => resource.id === 'go').version;
  if (args.includes('--check')) return { executed: false, steps, go: goVersion, engineBuild: 'go build -mod=readonly -trimpath ./cmd/attestara', engineReady: false };
  const binary = resolve(directory, 'dist/engine/attestara');
  const pending = binary + '.pending';
  mkdirSync(resolve(directory, 'dist/engine'), { recursive: true });
  rmSync(binary, { force: true }); rmSync(pending, { force: true });
  const env = { ...process.env, GOTOOLCHAIN: 'local', CGO_ENABLED: '0' };
  try {
    const probe = run('go', ['env', 'GOVERSION'], { cwd: directory, env, encoding: 'utf8' });
    if (probe.error || probe.status !== 0 || probe.stdout?.trim() !== 'go' + goVersion) throw new Error(`Setup failed: Go ${goVersion} is required on PATH; use the C02 source-build guide.`);
    for (const step of steps) {
      if (!process.env.npm_execpath) throw new Error('Run this command through npm run setup.');
      const result = run(process.execPath, [process.env.npm_execpath, ...step], { cwd: directory, stdio: 'inherit' });
      if (result.error || result.status !== 0) throw new Error('Setup failed at npm ' + step.join(' '));
    }
    const built = run('go', ['build', '-mod=readonly', '-trimpath', '-o', pending, './cmd/attestara'], { cwd: directory, env, stdio: 'inherit' });
    if (built.error || built.status !== 0) throw new Error('Setup failed building the Go engine.');
    const response = run(pending, ['--version'], { cwd: directory, encoding: 'utf8' });
    if (response.error || response.status !== 0) throw new Error('Setup failed probing the built engine.');
    validateEngineHealth(JSON.parse(response.stdout));
    renameSync(pending, binary);
  } catch (error) {
    rmSync(pending, { force: true }); rmSync(binary, { force: true }); throw error;
  }
  return { executed: true, steps, capability: 'local-engine-process', enginePrepared: true, coreReady: false };
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.argv[2] === 'doctor') {
      let built = false; try { await access(new URL('../dist/web/index.html', import.meta.url)); built = true; } catch {}
      const state = JSON.parse(await readFile(new URL('../plans/project-state.json', import.meta.url), 'utf8'));
      let enginePrepared = false;
      try {
        const probe = spawnSync(resolve(root, 'dist/engine/attestara'), ['--version'], { encoding: 'utf8' });
        if (probe.status === 0) { validateEngineHealth(JSON.parse(probe.stdout)); enginePrepared = true; }
      } catch {}
      console.log(JSON.stringify({ nodeSupported: supportedNode(process.version), webAssetsBuilt: built, enginePrepared, readiness: state.readiness, coreReady: state.core_ready, start: 'npm start', next: 'npm run plan -- next' }, null, 2));
      if (!built || !enginePrepared || !supportedNode(process.version)) process.exitCode = 1;
    } else console.log(JSON.stringify({ ...setup(process.argv.slice(3)), start: 'npm start', note: 'Full product usability requires the C30 workflow; complete OSS v1 requires C40.' }, null, 2));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
