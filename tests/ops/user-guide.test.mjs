import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { startLocal } from '../../scripts/start-local.mjs';

test('the published guide CLI commands evaluate, verify, restart and restore real snapshots', { timeout: 30000 }, async t => {
  const dir = await mkdtemp(join(tmpdir(), 'attestara-c34-guide-'));
  let runtime;
  t.after(async () => { if (runtime) await runtime.close(); await rm(dir, { recursive: true, force: true }); });
  const guide = await readFile(new URL('../../docs/release/USER_GUIDE.md', import.meta.url), 'utf8');
  const commands = guide.split('\n').filter(line => line.startsWith('dist/engine/attestara '));
  assert.equal(commands.length, 7, 'Review this integration when the documented workflow changes.');
  const replacements = new Map([
    ['.local-data/attestara', join(dir, 'store')],
    ['.local-data/guide-passport.json', join(dir, 'passport.json')],
    ['.local-data/guide-backup', join(dir, 'backup')],
    ['.local-data/guide-restored', join(dir, 'restored')],
  ]);
  runtime = await startLocal({ port: 0, store: join(dir, 'store') });
  for (let index = 0; index < commands.length; index++) {
    if (index === 4) { await runtime.close(); runtime = await startLocal({ port: 0, store: join(dir, 'store') }); }
    let command = commands[index];
    for (const [before, after] of replacements) command = command.replaceAll(before, after);
    const [invocation, output] = command.split(' > ');
    const [executable, ...args] = invocation.split(' ');
    // Execute fixed repository CLI arguments directly; never interpret docs/user bytes as shell.
    const result = spawnSync(resolve(executable), args, { encoding: 'utf8', timeout: 10000 });
    assert.equal(result.status, 0, `Guide step ${index + 1}: ${result.stderr}`);
    if (output) await writeFile(output, result.stdout);
  }
  const passport = JSON.parse(await readFile(join(dir, 'passport.json'), 'utf8'));
  assert.ok(passport, 'Evaluation must produce actual JSON before restore can be verified.');
});
