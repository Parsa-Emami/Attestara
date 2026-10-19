import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, readFile, writeFile, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { safeName, inventory, verifyInventory, publish } from '../../scripts/package-release.mjs';

test('release paths reject traversal, option injection and control characters', () => {
  for (const name of ['../x', '/x', '-x', 'a/../b', 'a\\b', 'a\nx', 'a//b']) assert.throws(() => safeName(name));
  assert.equal(safeName('internal/schema.json'), 'internal/schema.json');
});
test('release inventory detects altered bytes and rejects symlinks', async t => {
  const root = await mkdtemp(join(tmpdir(), 'c31-inventory-'));
  const { rm } = await import('node:fs/promises');
  t.after(() => rm(root, { recursive: true, force: true }));
  await writeFile(join(root, 'file'), 'original');
  const files = await inventory(root);
  await verifyInventory(root, files);
  await writeFile(join(root, 'file'), 'modified');
  await assert.rejects(verifyInventory(root, files), /checksum/);
  await symlink('/etc/passwd', join(root, 'link'));
  await assert.rejects(inventory(root), /regular/);
});
test('publication preserves every existing destination', async t => {
  const root = await mkdtemp(join(tmpdir(), 'c31-publish-'));
  const { rm } = await import('node:fs/promises');
  t.after(() => rm(root, { recursive: true, force: true }));
  const stage = join(root, 'stage'), target = join(root, 'target');
  await mkdir(stage); await writeFile(join(stage, 'marker'), 'complete');
  await mkdir(target);
  await assert.rejects(publish(stage, target), /exists/);
  assert.equal(await readFile(join(stage, 'marker'), 'utf8'), 'complete');
  await rm(target, { recursive: true });
  await publish(stage, target);
  assert.equal(await readFile(join(target, 'marker'), 'utf8'), 'complete');
});

test('archive write failures never produce a successful artifact', async t => {
  const { archive } = await import('../../scripts/package-release.mjs');
  const { rm } = await import('node:fs/promises');
  const root = await mkdtemp(join(tmpdir(), 'c31-full-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  await writeFile(join(root, 'file'), 'source');
  const output = join(root, 'archive.gz'); await symlink('/dev/full', output);
  await assert.rejects(archive(root, [{ path: 'file' }], output, 0), /Command failed/);
});

test('recorded inventories reject executable changes and symlink parents', async t => {
  const { rm, chmod } = await import('node:fs/promises');
  const root = await mkdtemp(join(tmpdir(), 'c31-parents-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  await mkdir(join(root, 'real')); await writeFile(join(root, 'real/file'), 'bytes');
  const files = await inventory(root);
  await chmod(join(root, 'real/file'), 0o755);
  await assert.rejects(verifyInventory(root, files), /checksum/);
  await chmod(join(root, 'real/file'), 0o644);
  await symlink(join(root, 'real'), join(root, 'link'));
  await assert.rejects(verifyInventory(root, [{ ...files[0], path: 'link/file' }]), /directory/);
});

test('web-only packaging preserves its standalone service entrypoint', async t => {
  const { execFileSync } = await import('node:child_process');
  const { cp, rm } = await import('node:fs/promises');
  const { fileURLToPath } = await import('node:url');
  const root = await mkdtemp(join(tmpdir(), 'c31-web-only-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const path of ['ops/web', 'scripts/serve-web.mjs', 'apps/web/src/lib/runtime-config.mjs', 'apps/web/src/lib/engine-health.mjs', 'LICENSE', 'NOTICE', 'docs/engineering/WEB_RUNBOOK.md']) {
    await mkdir(join(root, path, '..'), { recursive: true });
    await cp(new URL('../../' + path, import.meta.url), join(root, path), { recursive: true });
  }
  await mkdir(join(root, 'dist/web'), { recursive: true });
  await writeFile(join(root, 'dist/web/index.html'), '<h1>Synthetic web fixture</h1>');
  execFileSync('git', ['init', '--quiet', root]);
  execFileSync('git', ['-c', 'user.name=C31 fixture', '-c', 'user.email=c31@example.invalid', 'commit', '--allow-empty', '-qm', 'fixture'], { cwd: root });
  execFileSync(process.execPath, [fileURLToPath(new URL('../../scripts/package-web.mjs', import.meta.url))], { cwd: root, encoding: 'utf8' });
  const unit = await readFile(join(root, 'dist/release/ops/web/service.systemd'), 'utf8');
  assert.match(unit, /ExecStart=.*scripts\/serve-web\.mjs --root web/);
  assert.doesNotMatch(unit, /start-local|StateDirectory/);
  const metadata = JSON.parse(await readFile(join(root, 'dist/release/release.json'), 'utf8'));
  assert.equal(metadata.scope, 'web-foundation');
  assert.equal(metadata.engineReady, false);
});
