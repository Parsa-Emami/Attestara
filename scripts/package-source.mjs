import { readFile, writeFile, mkdir, lstat, chmod } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { join } from 'node:path';
import { safeName, sha256, verifyInventory } from './package-release.mjs';

const git = (root, args) => execFileSync('git', args, { cwd: root, maxBuffer: 32 * 1024 * 1024 });
export async function sourceSnapshot(root) {
  // A distributed source archive has no .git. Recheck every recorded input instead.
  try { await lstat(join(root, '.git')); } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    const source = JSON.parse(await readFile(join(root, 'SOURCE_MANIFEST.json'), 'utf8'));
    if (source.schemaVersion !== 1 || !/^[a-f0-9]{40}$/.test(source.sourceCommit) || !Number.isSafeInteger(source.epoch) || source.epoch < 0) throw new Error('Invalid source manifest');
    await verifyInventory(root, source.files);
    return source;
  }
  if (git(root, ['status', '--porcelain', '--untracked-files=normal']).length) throw new Error('Package requires a clean source snapshot');
  const sourceCommit = git(root, ['rev-parse', 'HEAD']).toString().trim();
  const epoch = Number(git(root, ['show', '-s', '--format=%ct', 'HEAD']).toString().trim());
  const files = [], excluded = [];
  const entries = git(root, ['ls-tree', '-rz', '--full-tree', 'HEAD']).toString().split('\0').filter(Boolean);
  for (const entry of entries) {
    const [header, name] = entry.split('\t');
    const [mode, kind] = header.split(' ');
    safeName(name);
    if (kind !== 'blob' || !['100644', '100755'].includes(mode)) throw new Error('Source must contain only regular files: ' + name);
    if (name.startsWith('.agents/skills/')) {
      const directory = name.split('/').slice(0, 3).join('/');
      if (!entries.some(item => item.endsWith('\t' + directory + '/LICENSE'))) {
        excluded.push({ path: name, reason: 'Optional copied agent skill has no declared upstream redistribution license' });
        continue;
      }
    }
    const bytes = await readFile(join(root, name));
    // Check the working bytes too, including changes hidden by assume-unchanged.
    const expected = git(root, ['show', sourceCommit + ':' + name]);
    if (!bytes.equals(expected)) throw new Error('Source differs from commit: ' + name);
    files.push({ path: name, sha256: sha256(bytes), mode: mode === '100755' ? 0o755 : 0o644 });
  }
  return { schemaVersion: 1, sourceCommit, epoch, files, excluded };
}

export async function copySource(root, destination, source) {
  for (const file of source.files) {
    safeName(file.path);
    const bytes = await readFile(join(root, file.path));
    if (sha256(bytes) !== file.sha256) throw new Error('Source changed during packaging');
    const target = join(destination, file.path);
    await mkdir(join(target, '..'), { recursive: true });
    await writeFile(target, bytes, { mode: file.mode }); await chmod(target, file.mode);
  }
  await writeFile(join(destination, 'SOURCE_MANIFEST.json'), JSON.stringify(source, null, 2) + '\n');
}
