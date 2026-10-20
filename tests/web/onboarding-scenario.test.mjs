import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { validateOnboardingEnvironment, validateOnboardingScenario, validateWorkflowScenario } from '../../scripts/clone-workflow.mjs';

const root = fileURLToPath(new URL('../../', import.meta.url));
const digest = bytes => 'sha256:' + createHash('sha256').update(bytes).digest('hex');

test('C37 acceptance audit pins reused fixtures and existing regression checks', async () => {
  const base = join(root, 'tests/fixtures/attestara/C37');
  const provenance = JSON.parse(await readFile(join(base, 'provenance.json'), 'utf8'));
  const cases = JSON.parse(await readFile(join(base, 'cases.json'), 'utf8'));
  assert.equal(cases.schema_version, 1);
  assert.equal(cases.license, 'Apache-2.0');
  assert.equal(provenance.license, cases.license);
  assert.equal(cases.profile, 'local-cli-snapshots');
  assert.deepEqual(cases.cases.map(item => item.id), ['prerequisites', 'malformed-input', 'core-workflow', 'restart-restore', 'accessible-viewer', 'release-integrity-trust', 'upgrade-rollback', 'provider-unavailable', 'inert-source']);
  for (const [path, expected] of Object.entries(provenance.files)) assert.equal(digest(await readFile(join(base, path))), expected, path);
  for (const [path, expected] of Object.entries(provenance.reused_inputs)) assert.equal(digest(await readFile(join(root, path))), expected, path);
  for (const item of cases.cases) {
    assert.ok(item.expected.length > 0);
    assert.ok(item.checks.length > 0);
    for (const path of item.checks) assert.ok((await readFile(join(root, path))).length > 0, path);
  }
});

test('C36 fixture provenance and positive/negative onboarding cases are pinned', async () => {
  const base = join(root, 'tests/fixtures/attestara/C36');
  const provenance = JSON.parse(await readFile(join(base, 'provenance.json'), 'utf8'));
  const cases = JSON.parse(await readFile(join(base, 'cases.json'), 'utf8'));
  assert.equal(provenance.license, 'Apache-2.0');
  assert.equal(cases.schema_version, 1);
  assert.equal(cases.license, provenance.license);
  assert.deepEqual(cases.cases.map(item => item.id), ['clean-guide-workflow', 'missing-prerequisite', 'malformed-onboarding-contract', 'restart-and-restore', 'inert-user-source', 'optional-providers']);
  for (const [path, expected] of Object.entries(provenance.files)) assert.equal(digest(await readFile(join(base, path))), expected, `C36 fixture hash mismatch: ${path}`);
});

test('C36 contract keeps the C01 specification historical and binds the public guide to the real C30 workflow', async () => {
  const scenario = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
  const onboarding = validateOnboardingScenario(scenario);
  assert.equal(scenario.steps.length, 10);
  assert.equal(scenario.status, 'unexecuted reference contract');
  assert.equal(onboarding.guide, 'docs/release/USER_GUIDE.md');
  assert.equal(onboarding.guide_test, 'tests/ops/user-guide.test.mjs');
  assert.equal(onboarding.user_source_execution, 'forbidden');
});

test('C36 rejects malformed scenario contracts before setup can appear successful', async () => {
  const scenario = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
  for (const mutate of [
    value => { value.onboarding.version = 2; },
    value => { delete value.onboarding.guide_test; },
    value => { value.onboarding.required_phases.pop(); },
    value => { value.onboarding.optional_providers.push('opa'); },
    value => { value.onboarding.user_source_execution = 'allowed'; },
  ]) {
    const changed = structuredClone(scenario);
    mutate(changed);
    assert.throws(() => validateOnboardingScenario(changed));
  }
});

test('shared CLI workflow accepts the historical C34 scenario without the later C36 onboarding contract', async () => {
  const scenario = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
  delete scenario.onboarding;
  assert.doesNotThrow(() => validateWorkflowScenario(scenario));
  const current = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
  assert.doesNotThrow(() => validateWorkflowScenario(current));
  current.onboarding.version = 2;
  assert.throws(() => validateWorkflowScenario(current), /Unsupported onboarding contract version/);
});

test('C36 reports missing or incompatible pinned prerequisites explicitly', async () => {
  const scenario = JSON.parse(await readFile(join(root, 'tests/scenarios/clone-to-value.json'), 'utf8'));
  const onboarding = validateOnboardingScenario(scenario);
  const actual = { node: '24.19.0', npm: '11.9.0', go: '1.27.2', platform: 'linux-x64' };
  assert.doesNotThrow(() => validateOnboardingEnvironment(onboarding, actual));
  for (const tool of ['node', 'npm', 'go', 'platform']) {
    const missing = { ...actual, [tool]: undefined };
    assert.throws(() => validateOnboardingEnvironment(onboarding, missing), /C36 prerequisite unavailable/);
    const wrong = { ...actual, [tool]: 'unsupported' };
    assert.throws(() => validateOnboardingEnvironment(onboarding, wrong), /C36 prerequisite unavailable/);
  }
});
