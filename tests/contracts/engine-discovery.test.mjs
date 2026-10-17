import test from 'node:test';
import assert from 'node:assert/strict';
import { suites, validateDiscovery } from '../../scripts/verify-engine-tests.mjs';

test('engine discovery requires every representative test in each real Go package', () => {
  for (const [pkg, names] of Object.entries(suites)) {
    assert.doesNotThrow(() => validateDiscovery(pkg, [...names, 'ok attestara/package'].join('\n')));
    assert.doesNotThrow(() => validateDiscovery(pkg, names.join('\r\n')));
    for (const missing of names) {
      assert.throws(() => validateDiscovery(pkg, names.filter(name => name !== missing).join('\n')), error => error.message.includes(pkg) && error.message.includes(missing));
    }
    for (const output of ['', '? attestara/package [no test files]', 'ok attestara/package [no tests to run]', names.map(name => 'prefix' + name).join('\n')]) {
      assert.throws(() => validateDiscovery(pkg, output), /required engine tests missing/);
    }
  }
});
