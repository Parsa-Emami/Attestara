import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setup, supportedNode } from '../../scripts/oss-bootstrap.mjs';
test('setup validates toolchain/options and stops at the first failed prerequisite', async t => {
  const dir = await mkdtemp(join(tmpdir(), 'oss-setup-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  assert.equal(supportedNode('v24.19.0'), true); assert.equal(supportedNode('v25.1.0'), false);
  assert.throws(() => setup(['--unknown']), /options/);
  const prior = process.env.npm_execpath; process.env.npm_execpath ??= 'test-npm';
  let calls = 0; assert.throws(() => setup([], () => { calls++; return { status: 1 }; }, process.version, dir), /failed/); assert.equal(calls, 1);
  const check = setup(['--check'], (command, args) => {
    assert.equal(command, process.execPath);
    assert.deepEqual(args, [process.env.npm_execpath, '--version']);
    return { status: 0, stdout: '11.9.0\n' };
  });
  assert.equal(check.executed, false); assert.equal(check.npm, '11.9.0');
  assert.throws(() => setup(['--check'], () => ({ status: 0, stdout: '11.8.0\n' })), /npm 11.9.0/);
  if (prior === undefined) delete process.env.npm_execpath; else process.env.npm_execpath = prior;
});
