import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { cpSync, readFileSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { createHash } from 'node:crypto';

export const workflowCommands = [];

export function validateOnboardingScenario(scenario) {
  const onboarding = scenario?.onboarding;
  assert.equal(scenario?.version, 1, 'Unsupported clone-to-value scenario version');
  assert.equal(scenario?.status, 'unexecuted reference contract', 'C01 reference status is historical');
  assert.equal(scenario?.engine_execution_verified, false, 'C01 scenario cannot certify engine execution');
  assert.equal(scenario?.execution?.profile, 'local-cli-snapshots', 'Unsupported onboarding profile');
  assert.equal(onboarding?.version, 1, 'Unsupported onboarding contract version');
  assert.equal(onboarding.guide, 'docs/release/USER_GUIDE.md', 'Onboarding must follow the public user guide');
  assert.equal(onboarding.guide_test, 'tests/ops/user-guide.test.mjs', 'Missing literal guide regression');
  assert.equal(onboarding.browser_test, 'tests/browser/clone-to-value.spec.ts', 'Missing core browser workflow');
  assert.deepEqual(onboarding.required_phases, ['prerequisites', 'setup', 'guide-cli-restart-restore', 'browser-install', 'service-start', 'cli-workflow', 'service-restart', 'new-store-restore', 'browser-viewer'], 'Onboarding phases are incomplete or reordered');
  assert.deepEqual(onboarding.toolchain, { node: '24.19.0', npm: '11.9.0', go: '1.27.2', platform: 'linux-x64' }, 'Unsupported onboarding toolchain');
  assert.deepEqual(onboarding.optional_providers, [], 'Optional providers cannot become onboarding prerequisites');
  assert.equal(onboarding.user_source_execution, 'forbidden', 'User source execution must remain forbidden');
  return onboarding;
}

export function validateOnboardingEnvironment(onboarding, actual) {
  for (const tool of ['node', 'npm', 'go']) {
    assert.equal(actual?.[tool], onboarding.toolchain[tool], `C36 prerequisite unavailable: ${tool}`);
  }
  assert.equal(actual?.platform, onboarding.toolchain.platform, 'C36 prerequisite unavailable: linux-x64');
}

export function engineCommand(root, args, code = 0) {
  const started = performance.now();
  const result = spawnSync(join(root, 'dist/engine/attestara'), args, { cwd: root, encoding: 'utf8', timeout: 20000, maxBuffer: 2 * 1024 * 1024 });
  workflowCommands.push({ command: ['attestara', ...args], expected_exit: code, exit: result.status, duration_ms: Math.round(performance.now() - started), stdout: result.stdout, stderr: result.stderr });
  assert.equal(result.error, undefined, 'engine invocation failed');
  assert.equal(result.status, code, `${args[0]}: ${result.stderr}`);
  return result;
}

export function runWorkflow(root, work, store = join(work, 'store')) {
  const fixture = join(root, 'tests/fixtures/attestara/C30');
  const manifest = join(fixture, 'manifest.json');
  const scenario = JSON.parse(readFileSync(join(root, 'tests/scenarios/clone-to-value.json')));
  assert.equal(scenario.steps.length, 10);
  validateOnboardingScenario(scenario);
  const provenance = JSON.parse(readFileSync(join(fixture, 'provenance.json')));
  assert.equal(provenance.license, 'Apache-2.0');
  for (const [path, digest] of Object.entries(provenance.files)) assert.equal('sha256:' + createHash('sha256').update(readFileSync(join(fixture, path))).digest('hex'), digest);
  const args = ['evaluate', '--input', manifest, '--policy', join(fixture, 'rule.json'), '--evidence', join(fixture, 'evidence.json'), '--freshness', 'fresh', '--files', fixture, '--store', store, '--operation-id', 'release-1'];
  const first = engineCommand(root, args).stdout;
  const passport = join(work, 'passport.json');
  writeFileSync(passport, first, { mode: 0o600 });
  assert.equal(JSON.parse(first).policy_decision, 'pass');
  assert.equal(JSON.parse(first).verification, 'unverified');
  assert.equal(engineCommand(root, args).stdout, first, 'identical operation replay');
  const stale = [...args]; stale[8] = 'stale';
  engineCommand(root, stale, 3);
  assert.equal(engineCommand(root, ['verify', '--store', store, '--operation-id', 'release-1']).stdout, first);
  const inspect = ['verify', '--input', manifest, '--files', fixture, '--passport', passport];
  assert.equal(engineCommand(root, inspect).stdout, first);
  const trusted = engineCommand(root, [...inspect, '--signature', join(fixture, 'signature.json'), '--trust', join(fixture, 'trust.json')]);
  assert.equal(JSON.parse(trusted.stderr).manifest_trusted, true);
  assert.equal(trusted.stdout, first);
  const untrusted = engineCommand(root, [...inspect, '--signature', join(fixture, 'signature.json'), '--trust', join(fixture, 'untrusted.json')]);
  assert.equal(JSON.parse(untrusted.stderr).manifest_trusted, false);
  engineCommand(root, [...inspect, '--signature', join(fixture, 'invalid-signature.json')], 2);
  const changed = join(work, 'changed'); cpSync(fixture, changed, { recursive: true });
  const subject = readFileSync(join(changed, 'subject.txt')); subject[0] ^= 1; writeFileSync(join(changed, 'subject.txt'), subject);
  engineCommand(root, ['verify', '--input', manifest, '--files', changed, '--passport', passport], 2);
  const incomplete = JSON.parse(readFileSync(manifest)); delete incomplete.evidence_digest;
  writeFileSync(join(work, 'incomplete.json'), JSON.stringify(incomplete));
  const invalid = [...args]; invalid[2] = join(work, 'incomplete.json'); invalid[14] = 'invalid';
  engineCommand(root, invalid, 2);
  const archive = join(work, 'release.zip');
  engineCommand(root, ['export', '--input', manifest, '--files', fixture, '--output', archive]);
  assert.equal(engineCommand(root, ['verify', '--archive', archive, '--passport', passport]).stdout, first);
  const backup = join(work, 'backup');
  assert.equal(engineCommand(root, ['export', '--store', store, '--operation-id', 'release-1', '--output', backup]).stdout, first);
  return { store, backup, passport, first };
}

export function restoreWorkflow(root, work, state) {
  assert.equal(engineCommand(root, ['verify', '--store', state.store, '--operation-id', 'release-1']).stdout, state.first);
  const restored = join(work, 'restored');
  assert.equal(engineCommand(root, ['restore', '--backup', state.backup, '--store', restored, '--operation-id', 'release-1']).stdout, state.first);
  engineCommand(root, ['restore', '--backup', state.backup, '--store', state.store, '--operation-id', 'release-1'], 3);
  const corrupt = join(work, 'corrupt'); cpSync(state.backup, corrupt, { recursive: true });
  writeFileSync(join(corrupt, 'manifest'), 'corrupted');
  const rejected = join(work, 'rejected');
  engineCommand(root, ['restore', '--backup', corrupt, '--store', rejected, '--operation-id', 'release-1'], 3);
  assert.equal(existsSync(rejected), false);
  return engineCommand(root, ['verify', '--store', restored, '--operation-id', 'release-1']).stdout;
}
