import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const read = async path => JSON.parse(await readFile(new URL('../../' + path, import.meta.url), 'utf8'));

test('C04 versioned schemas and synthetic concrete artifacts preserve product requirements', async () => {
  const [input, result, product, positive, expected, provenance] = await Promise.all([
    'contracts/input.schema.json', 'contracts/result.schema.json', 'product/project.json',
    'tests/fixtures/attestara/C04/input.valid.json', 'tests/fixtures/attestara/C04/expected.json',
    'tests/fixtures/attestara/C04/provenance.json',
  ].map(read));
  assert.equal(input.properties.schema_version.const, '1');
  assert.equal(result.properties.schema_version.const, '1');
  assert.equal(input.additionalProperties, false);
  assert.equal(result.additionalProperties, false);
  for (const name of Object.keys(product.domain.fields)) assert.ok(input.required.includes(name));
  assert.equal(input.properties.files.maxItems, product.limits.files_per_bundle);
  assert.equal(input.properties.files.items.properties.size_bytes.maximum, product.limits.bundle_max_mib * 1024 ** 2);
  assert.equal(provenance.license, 'Apache-2.0');
  assert.deepEqual(expected.required_providers, []);
  assert.equal(expected.input_digest, 'sha256:' + createHash('sha256').update(await readFile(new URL('../fixtures/attestara/C04/input.valid.json', import.meta.url))).digest('hex'));
  assert.equal(expected.verification, 'unverified');
  assert.equal(expected.policy_decision, 'unknown');
  for (const file of positive.files) {
    assert.ok(provenance.files.includes(file.path));
    const bytes = await readFile(new URL('../fixtures/attestara/C04/' + file.path, import.meta.url));
    assert.equal(file.digest, 'sha256:' + createHash('sha256').update(bytes).digest('hex'));
    assert.equal(file.size_bytes, bytes.length);
  }
  for (const name of ['input', 'result']) {
    const draft = await read(`contracts/drafts/${name}.v0.schema.json`);
    assert.match(draft.title, /draft/);
  }
});

test('C04 Node entry exercises actual Go validators, including hostile/version/size/cancellation cases', () => {
  const output = execFileSync('go', ['test', '-count=1', '-v', './internal/contracts'], { cwd: root, encoding: 'utf8', timeout: 120000, maxBuffer: 1024 * 1024 });
  for (const name of ['TestC04Fixtures', 'TestC04StrictJSON', 'TestC04LimitsAndCancellation', 'TestC04DraftCompatibility']) assert.ok(output.includes('--- PASS: ' + name), name);
});

test('C04 fresh engine test processes reproduce immutable fixture expectations after restart', () => {
  for (let attempt = 0; attempt < 2; attempt++) {
    const output = execFileSync('go', ['test', '-count=1', '-run', '^TestC04Fixtures$', './internal/contracts'], { cwd: root, encoding: 'utf8', timeout: 120000 });
    assert.match(output, /^ok\s+attestara\/internal\/contracts/m);
  }
});
