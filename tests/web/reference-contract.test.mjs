import test from 'node:test';
import assert from 'node:assert/strict';
import { cp, mkdtemp, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { verifyReference } from '../../scripts/verify-reference.mjs';

const sourceRoot = fileURLToPath(new URL('../../', import.meta.url));
const scenarioPath = 'tests/scenarios/clone-to-value.json';
const guidePath = 'docs/product/REFERENCE_WORKFLOW.md';

async function copyContract(t) {
  const root = await mkdtemp(join(tmpdir(), 'attestara-c01-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const path of [scenarioPath, guidePath, 'product/project.json']) {
    const target = join(root, path);
    await cp(join(sourceRoot, path), target, { recursive: true });
  }
  await cp(join(sourceRoot, 'tests/fixtures/attestara/C01'), join(root, 'tests/fixtures/attestara/C01'), { recursive: true });
  return root;
}

async function changeScenario(root, change) {
  const path = join(root, scenarioPath);
  const scenario = JSON.parse(await readFile(path, 'utf8'));
  change(scenario);
  await writeFile(path, JSON.stringify(scenario));
}

test('C01 accepts its synthetic reference contract without executing the engine', async t => {
  const root = await copyContract(t);
  assert.deepEqual(await verifyReference({ root }), { fixtures: 3, steps: 10 });
});

test('C01 rejects missing actions, results, invariants and fixture references', async t => {
  for (const change of [
    s => { delete s.steps[0].action; },
    s => { delete s.steps[0].expected_result; },
    s => { s.invariants = []; },
    s => { delete s.steps[0].fixture_refs; },
    s => { s.steps[0].fixture_refs = ['absent']; },
  ]) {
    const root = await copyContract(t);
    await changeScenario(root, change);
    await assert.rejects(verifyReference({ root }), /action|expected_result|invariants|fixture/i);
  }
});

test('C01 rejects absent provenance or license and invalid fixture JSON', async t => {
  for (const field of ['provenance', 'license']) {
    const root = await copyContract(t);
    const path = join(root, 'tests/fixtures/attestara/C01/positive.json');
    const fixture = JSON.parse(await readFile(path, 'utf8'));
    delete fixture[field];
    await writeFile(path, JSON.stringify(fixture));
    await assert.rejects(verifyReference({ root }), /provenance|license/i);
  }
  const root = await copyContract(t);
  await writeFile(join(root, 'tests/fixtures/attestara/C01/positive.json'), '{');
  await assert.rejects(verifyReference({ root }), /JSON|Unexpected/i);
});

test('C01 rejects missing expected outputs and incorrect domain digests', async t => {
  for (const change of [
    f => { f.outcomes.pop(); },
    f => { f.subject_digest = 'sha256:' + '0'.repeat(64); },
  ]) {
    const root = await copyContract(t);
    const path = join(root, 'tests/fixtures/attestara/C01/expected.json');
    const fixture = JSON.parse(await readFile(path, 'utf8'));
    change(fixture);
    await writeFile(path, JSON.stringify(fixture));
    await assert.rejects(verifyReference({ root }), /outcomes|subject_digest/i);
  }
});

test('C01 rejects path escapes, unsupported versions and false engine claims', async t => {
  for (const path of ['../outside.json', '/tmp/outside.json', 'tests\\fixtures\\..\\outside.json']) {
    const root = await copyContract(t);
    await changeScenario(root, s => { s.fixtures[0].path = path; });
    await assert.rejects(verifyReference({ root }), /fixture path/i);
  }
  {
    const root = await copyContract(t);
    await changeScenario(root, s => { s.version = 2; });
    await assert.rejects(verifyReference({ root }), /version/i);
  }
  {
    const root = await copyContract(t);
    await changeScenario(root, s => { s.engine_execution_verified = true; });
    await assert.rejects(verifyReference({ root }), /engine/i);
  }
  {
    const root = await copyContract(t);
    const path = join(root, guidePath);
    await writeFile(path, (await readFile(path, 'utf8')) + '\nEngine passed the reference workflow.\n');
    await assert.rejects(verifyReference({ root }), /engine/i);
  }
});

test('C01 rejects symlinks leaving the fixture directory', async t => {
  const root = await copyContract(t);
  const path = join(root, 'tests/fixtures/attestara/C01/positive.json');
  const outside = join(root, 'outside.json');
  await cp(path, outside);
  await rm(path);
  await symlink(outside, path);
  await assert.rejects(verifyReference({ root }), /fixture path/i);
});
