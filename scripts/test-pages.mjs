import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { readFile, realpath, stat, mkdir, writeFile } from 'node:fs/promises';
import { join, resolve, relative, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from '@playwright/test';

const root = fileURLToPath(new URL('../', import.meta.url));
const output = join(root, 'dist/pages');
const diagnostics = join(root, 'dist/pages-check');
await mkdir(diagnostics, { recursive: true });
const report = { schema_version: 1, scope: 'github-pages-static-viewer', passed: false, engine_hosted: false, public_installation_verified: false, checks: [], blockers: [] };
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.webmanifest': 'application/manifest+json', '.svg': 'image/svg+xml' };
const server = createServer(async (request, response) => {
  try {
    assert.ok(['GET', 'HEAD'].includes(request.method));
    const path = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
    assert.ok(path.startsWith('/Attestara/'));
    const file = await realpath(resolve(output, path.slice('/Attestara/'.length) || 'index.html'));
    assert.ok(!relative(output, file).startsWith('..') && (await stat(file)).isFile());
    const bytes = await readFile(file);
    response.writeHead(200, { 'Content-Type': types[extname(file)] ?? 'application/octet-stream' });
    response.end(request.method === 'HEAD' ? undefined : bytes);
  } catch { response.writeHead(404); response.end('Not found'); }
});
let browser;
try {
  const manifest = JSON.parse(await readFile(join(output, 'manifest.webmanifest')));
  assert.equal(manifest.scope, './'); assert.equal(manifest.start_url, './'); assert.equal(manifest.icons[0].src, './icon.svg');
  const config = JSON.parse(await readFile(join(output, 'runtime-config.json'))); assert.equal(config.engineEnabled, false);
  const deployment = JSON.parse(await readFile(join(output, 'deployment.json'))); assert.equal(deployment.engine_hosted, false);
  report.source_commit = deployment.source_commit;
  report.source_dirty = deployment.source_dirty;
  await new Promise(accept => server.listen(0, '127.0.0.1', accept));
  const origin = 'http://127.0.0.1:' + server.address().port;
  browser = await chromium.launch({ ...(process.env.WEB_TEST_CHROMIUM ? { executablePath: process.env.WEB_TEST_CHROMIUM } : {}), args: ['--no-sandbox'] });
  const page = await browser.newPage({ viewport: { width: 375, height: 812 } });
  const errors = [], requests = [], failures = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => requests.push({ url: request.url(), method: request.method() }));
  page.on('response', response => { if (response.status() >= 400) failures.push(response.url()); });
  await page.goto(origin + '/Attestara/');
  await page.getByRole('heading', { level: 1 }).waitFor();
  assert.match(await page.getByLabel('Availability').innerText(), /Online viewer/);
  assert.equal(await page.getByRole('link', { name: 'Download an example passport' }).getAttribute('href'), '/Attestara/example-passport.json');
  await page.getByRole('link', { name: 'Passport', exact: true }).click();
  await page.getByLabel('Choose passport file').setInputFiles(join(output, 'example-passport.json'));
  await page.getByRole('heading', { name: 'attestara-c14-release', exact: true }).waitFor();
  await page.getByText('Viewer assessment: Unverified', { exact: true }).waitFor();
  await page.getByRole('button', { name: 'Clear passport', exact: true }).click();
  await page.getByRole('link', { name: 'Workspace', exact: true }).click();
  await page.getByLabel('Choose JSON file').setInputFiles({ name: 'local-only.json', mimeType: 'application/json', buffer: Buffer.from('{"local_marker":"NEVER_UPLOAD_THIS_MARKER"}') });
  await page.getByRole('heading', { name: 'local-only.json', exact: true }).waitFor();
  await page.getByRole('link', { name: 'Roadmap', exact: true }).click();
  await page.getByLabel('Search tasks').fill('C40');
  assert.equal(await page.locator('.tasks > li').count(), 1);
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  await page.goto(origin + '/Attestara/#passport');
  await page.getByLabel('Choose passport file').waitFor();
  assert.equal(await page.locator('link[rel="manifest"]').getAttribute('href'), '/Attestara/manifest.webmanifest');
  assert.deepEqual(errors, []); assert.deepEqual(failures, []);
  assert.ok(requests.every(request => request.method === 'GET' && new URL(request.url).origin === origin && new URL(request.url).pathname.startsWith('/Attestara/') && !request.url.includes('/api/') && !request.url.includes('NEVER_UPLOAD_THIS_MARKER')));
  await page.screenshot({ path: join(diagnostics, 'viewer.png'), fullPage: true });
  report.checks = ['Real browser loaded the repository subpath and base-prefixed configuration/assets/manifest.', 'Existing passport import and unverified assessment, JSON preview, clear actions, roadmap filtering and direct hash navigation passed.', '375px reflow passed; selected bytes stayed in the tab; no API, external request, failed response or script error observed.'];
  report.passed = true;
} catch (error) { report.blockers.push(error.message); }
finally {
  await browser?.close();
  if (server.listening) await new Promise(accept => server.close(accept));
  await writeFile(join(diagnostics, 'report.json'), JSON.stringify(report, null, 2) + '\n');
}
console.log(JSON.stringify(report, null, 2));
process.exitCode = report.passed ? 0 : 1;
