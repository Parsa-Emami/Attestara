import { execFileSync } from 'node:child_process';
import { readFile, writeFile, cp, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const output = join(root, 'dist/pages');
const run = args => execFileSync(process.execPath, args, { cwd: root, stdio: 'inherit', env: { ...process.env, ATTESTARA_WEB_TARGET: 'github-pages' } });
run(['scripts/generate-web-data.mjs']);
run(['node_modules/typescript/bin/tsc', '-p', 'tsconfig.json']);
run(['node_modules/vite/bin/vite.js', 'build', '--config', 'apps/web/vite.config.ts', '--outDir', output]);

const manifest = JSON.parse(await readFile(join(output, 'manifest.webmanifest')));
manifest.id = './'; manifest.start_url = './'; manifest.scope = './';
manifest.icons = manifest.icons.map(icon => ({ ...icon, src: './icon.svg' }));
await writeFile(join(output, 'manifest.webmanifest'), JSON.stringify(manifest, null, 2) + '\n');
const index = await readFile(join(output, 'index.html'), 'utf8');
const policy = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; object-src 'none'";
await writeFile(join(output, 'index.html'), index.replace('<head>', `<head><meta http-equiv="Content-Security-Policy" content="${policy}"><meta name="referrer" content="no-referrer">`));
await writeFile(join(output, '.nojekyll'), '');
await cp(join(root, 'tests/fixtures/attestara/C19/expected.json'), join(output, 'example-passport.json'));
const commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
const dirty = execFileSync('git', ['status', '--porcelain', '--untracked-files=normal'], { cwd: root, encoding: 'utf8' }).trim() !== '';
await writeFile(join(output, 'deployment.json'), JSON.stringify({ schema_version: 1, source_commit: commit, source_dirty: dirty, scope: 'static-local-passport-viewer', engine_hosted: false, C40_verified: false }, null, 2) + '\n');
await mkdir(join(root, 'dist/pages-check'), { recursive: true });
console.log('Built GitHub Pages viewer at /Attestara/; local engine and signed-release status are unchanged.');
