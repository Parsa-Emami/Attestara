import { readFile, realpath } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { isAbsolute, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const projectRoot = fileURLToPath(new URL('../', import.meta.url));
const requiredKinds = ['positive', 'negative', 'expected-output'];
const requiredFields = ['action', 'expected_state', 'expected_result', 'on_error', 'outcome_ref'];
const requiredSteps = ['load', 'policy', 'passport', 'changed-subject', 'incomplete-manifest', 'unsigned', 'invalid-signature', 'signed-untrusted', 'viewer-parity', 'restart-restore'];
const present = value => typeof value === 'string' && value.trim().length > 0;
const check = (condition, message) => { if (!condition) throw new Error(message); };
const digest = value => 'sha256:' + createHash('sha256').update(value, 'utf8').digest('hex');
const inside = (parent, child) => child === parent || child.startsWith(parent + sep);
const readJson = async path => JSON.parse(await readFile(path, 'utf8'));

export async function verifyReference({ root = projectRoot } = {}) {
  const rootReal = await realpath(root);
  const scenario = await readJson(join(rootReal, 'tests/scenarios/clone-to-value.json'));
  const guide = await readFile(join(rootReal, 'docs/product/REFERENCE_WORKFLOW.md'), 'utf8');
  const product = await readJson(join(rootReal, 'product/project.json'));
  check(scenario.version === 1, 'Unsupported scenario version');
  check(scenario.project === product.name && scenario.guide === 'docs/product/REFERENCE_WORKFLOW.md', 'Scenario identity or guide mismatch');
  check(scenario.status === 'unexecuted reference contract' && scenario.engine_execution_verified === false && scenario.passed !== true, 'Engine execution must remain unverified');
  check(/^Execution status: unimplemented\./m.test(guide) && !/^Engine (?:execution )?(?:passed|verified)\b/im.test(guide), 'Guide claims engine execution or omits unimplemented status');
  check(Array.isArray(scenario.invariants) && JSON.stringify(scenario.invariants) === JSON.stringify(product.domain.invariants), 'Missing or changed domain invariants');
  check(Array.isArray(scenario.steps) && JSON.stringify(scenario.steps.map(step => step.id)) === JSON.stringify(requiredSteps), 'Missing or reordered workflow steps');
  check(Array.isArray(scenario.fixtures) && scenario.fixtures.length === 3, 'Expected positive, negative and expected-output fixtures');

  const fixtureBase = await realpath(join(rootReal, 'tests/fixtures/attestara/C01'));
  check(inside(rootReal, fixtureBase), 'Unsafe fixture path');
  const fixtures = new Map();
  for (const ref of scenario.fixtures) {
    check(present(ref?.id) && requiredKinds.includes(ref.kind) && !fixtures.has(ref.id), 'Invalid or duplicate fixture reference');
    check(typeof ref.path === 'string' && /^tests\/fixtures\/attestara\/C01\/[a-z0-9-]+\.json$/.test(ref.path) && !isAbsolute(ref.path), 'Unsafe fixture path');
    const path = await realpath(resolve(rootReal, ref.path));
    check(inside(rootReal, path) && inside(fixtureBase, path), 'Unsafe fixture path');
    const fixture = await readJson(path);
    check(fixture.id === ref.id && fixture.kind === ref.kind, 'Fixture identity mismatch');
    check(present(fixture.provenance) && present(fixture.license), 'Fixture provenance/license missing');
    check(fixture.provenance.startsWith('repository-authored synthetic') && fixture.license === 'Apache-2.0', 'Fixture provenance/license unsupported');
    fixtures.set(ref.id, fixture);
  }
  check(JSON.stringify([...fixtures.values()].map(f => f.kind).sort()) === JSON.stringify([...requiredKinds].sort()), 'Fixture kinds incomplete');
  const positive = [...fixtures.values()].find(f => f.kind === 'positive');
  const negative = [...fixtures.values()].find(f => f.kind === 'negative');
  const expected = [...fixtures.values()].find(f => f.kind === 'expected-output');
  check([positive.owner, positive.release_id, positive.source_revision, positive.sbom?.format, positive.sbom?.description, positive.policy_intent].every(present), 'Positive fixture lacks domain identity');
  for (const [bytes, hash] of [['subject_bytes_utf8', 'subject_digest'], ['evidence_bytes_utf8', 'evidence_digest'], ['policy_bytes_utf8', 'policy_digest']]) {
    check(present(positive[bytes]) && positive[hash] === digest(positive[bytes]) && expected[hash] === positive[hash], `Invalid ${hash} binding`);
  }
  check(expected.release_id === positive.release_id && expected.source_revision === positive.source_revision && expected.verification === 'unverified', 'Expected output identity or verification mismatch');
  check(Array.isArray(expected.outcomes) && expected.outcomes.length === scenario.steps.length, 'Expected outcomes missing');
  const outcomes = new Map(expected.outcomes.map(item => [item.step_id, item]));
  check(outcomes.size === scenario.steps.length, 'Duplicate expected outcomes');
  check(Array.isArray(negative.cases) && negative.cases.length === 5 && present(negative.owner) && negative.source_revision === positive.source_revision, 'Negative fixture identity/cases missing');
  const negativeIds = new Set(negative.cases.map(item => item.step_id));
  check(negativeIds.size === negative.cases.length, 'Duplicate negative cases');
  for (const step of scenario.steps) {
    check(requiredFields.every(field => present(step[field])), `Step ${step.id}: missing action, expected result/state or error`);
    check(Array.isArray(step.fixture_refs) && step.fixture_refs.length >= 2 && step.fixture_refs.every(id => fixtures.has(id)) && step.fixture_refs.includes(expected.id), `Step ${step.id}: missing fixture reference`);
    check(step.fixture_refs.includes(negativeIds.has(step.id) ? negative.id : positive.id), `Step ${step.id}: wrong input fixture reference`);
    const outcome = outcomes.get(step.outcome_ref);
    check(step.outcome_ref === step.id && outcome?.step_id === step.id && outcome.summary === step.expected_result && present(outcome.status), `Step ${step.id}: expected output mismatch`);
  }
  for (const item of negative.cases) {
    const step = scenario.steps.find(s => s.id === item.step_id);
    check(step?.fixture_refs.includes(negative.id) && present(item.mutation) && item.expected_verification === outcomes.get(item.step_id)?.status, `Negative case ${item.step_id}: expected verification mismatch`);
  }
  return { fixtures: fixtures.size, steps: scenario.steps.length };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const result = await verifyReference();
    console.log(`Validated C01 reference contract: ${result.fixtures} synthetic fixtures, ${result.steps} unexecuted steps; no engine capability inferred.`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
