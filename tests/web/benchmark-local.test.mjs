import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { loadScenario } from '../../scripts/benchmark-local.mjs';

const scenario = new URL('../scenarios/target-workload.json', import.meta.url);

test('C28 scenario resolves only bounded repository fixtures', async t => {
  const source = JSON.parse(await readFile(scenario, 'utf8'));
  const loaded = await loadScenario(scenario);
  assert.equal(loaded.scenario.target_input_bytes, 100 * 1024 * 1024);
  assert.equal(Object.keys(loaded.resolved).length, 6);

  const dir = await mkdtemp(join(tmpdir(), 'attestara-c28-case-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, 'scenario.json');
  await writeFile(path, '{');
  await assert.rejects(loadScenario(path), /invalid JSON/);
  await writeFile(path, JSON.stringify({ ...source, target_input_bytes: 1 }));
  await assert.rejects(loadScenario(path), /input sizes/);
  await writeFile(path, JSON.stringify({ ...source, fixtures: { ...source.fixtures, input: '../../etc/passwd' } }));
  await assert.rejects(loadScenario(path), /invalid input path/);
  await writeFile(path, JSON.stringify({ ...source, fixtures: { ...source.fixtures, input: 'C28/missing.json' } }));
  await assert.rejects(loadScenario(path), /ENOENT/);
});

test('C28 benchmark reports a missing tool without publishing success', () => {
  const script = new URL('../../scripts/benchmark-local.mjs', import.meta.url);
  const result = spawnSync(process.execPath, [script.pathname], { encoding: 'utf8', env: { ...process.env, PATH: '/nonexistent' } });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /C28 benchmark failed.*spawn go ENOENT/);
  assert.equal(result.stdout, '');
});
