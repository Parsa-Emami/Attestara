import { expect, test, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';

const root = new URL('../fixtures/attestara/C27/', import.meta.url);
const cases = JSON.parse(await readFile(new URL('cases.json', root), 'utf8'));
const passportBytes = await readFile(new URL('passport.json', root));
const passport = JSON.parse(passportBytes.toString('utf8'));
const originalBytes = await readFile(new URL('../C21/positive.json', root));
const original = JSON.parse(originalBytes.toString('utf8'));
const upload = (buffer: Buffer, name = 'passport.json') => ({ name, mimeType: 'application/json', buffer });

async function localPage(page: Page, path: string) {
  await page.goto(path);
  await expect(page.getByLabel('Availability')).toContainText('healthy at the last check');
  const effects: string[] = [];
  page.on('request', request => {
    // Chromium may refresh the declared favicon on hash navigation.
    if (request.method() === 'GET' && request.url() === new URL('/icon.svg', page.url()).href && request.postData() === null) return;
    effects.push(`${request.method()} ${request.url()}`);
  });
  page.on('pageerror', error => effects.push(error.message));
  page.on('dialog', dialog => { effects.push(dialog.message()); void dialog.dismiss(); });
  return effects;
}

async function inert(page: Page, effects: string[]) {
  // Check inserted DOM as well as execution: CSP must not hide unsafe rendering.
  await expect(page.locator('main img, main svg, main script, main iframe, main a[href^="javascript:"]')).toHaveCount(0);
  expect(await page.evaluate(() => Reflect.get(window, '__c27Injected'))).toBeUndefined();
  expect(await page.evaluate(() => Reflect.get(Object.prototype, 'polluted'))).toBeUndefined();
  expect(effects).toEqual([]);
}

test('C27 fixtures retain versioned synthetic provenance and exact hashes', async () => {
  const provenance = JSON.parse(await readFile(new URL('provenance.json', root), 'utf8'));
  expect(cases.schema_version).toBe(1);
  expect(provenance.schema_version).toBe(1);
  expect(provenance.license).toBe('Apache-2.0');
  expect(cases.preview_negative).toHaveLength(2);
  expect(cases.passport_negative).toHaveLength(8);
  expect(cases.http_negative).toHaveLength(4);
  expect(cases.unsupported_authorization).toHaveLength(4);
  expect(Object.keys(provenance.files).sort()).toEqual([
    'tests/fixtures/attestara/C21/positive.json',
    ...['README.md', 'cases.json', 'passport.json'].map(name => `tests/fixtures/attestara/C27/${name}`),
  ].sort());
  for (const [path, hash] of Object.entries(provenance.files)) {
    const bytes = await readFile(new URL('../../' + path, import.meta.url));
    expect('sha256:' + createHash('sha256').update(bytes).digest('hex')).toBe(hash);
  }
  for (const group of [cases.preview_negative, cases.passport_negative, cases.http_negative]) {
    expect(new Set(group.map((entry: { id: string }) => entry.id)).size).toBe(group.length);
  }
});

test('C27 local preview escapes hostile names and keys without requests or prototype pollution', async ({ page }) => {
  const effects = await localPage(page, '/');
  const input = page.getByLabel('Choose JSON file');
  await input.setInputFiles(upload(Buffer.from(cases.preview.text), cases.preview.name));
  await expect(page.getByRole('status').getByRole('heading')).toHaveText(cases.preview.name);
  await expect(page.getByRole('status').locator('code')).toHaveText(cases.preview.field);
  await inert(page, effects);
  for (const entry of cases.preview_negative) {
    await test.step(entry.id, async () => {
      await input.setInputFiles(upload(Buffer.from(entry.text)));
      await expect(page.getByRole('alert')).toHaveText(entry.error);
      await expect(page.locator('.preview')).toHaveCount(0);
      await inert(page, effects);
    });
  }
  await page.getByRole('link', { name: 'Roadmap', exact: true }).click();
  await page.getByLabel('Search tasks').fill(cases.preview.field);
  await expect(page.getByText('No matching tasks.')).toBeVisible();
  await inert(page, effects);
});

test('C27 hostile passport fields stay literal and unverified without sending selected bytes', async ({ page }) => {
  const effects = await localPage(page, '/#passport');
  await page.getByLabel('Choose passport file').setInputFiles(upload(passportBytes));
  await expect(page.locator('article.preview h3').first()).toHaveText(passport.release_id);
  await expect(page.locator('.passport-details').getByText(passport.source_revision, { exact: true })).toBeVisible();
  await expect(page.locator('article.preview li')).toHaveText(passport.warnings);
  await expect(page.getByText('Viewer assessment: Unverified')).toBeVisible();
  await inert(page, effects);
  await page.getByRole('button', { name: 'Clear passport' }).click();
  await expect(page.locator('article.preview')).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('status')).toHaveText('No passport selected.');
});

test('C27 invalid passports discard the previous display and fail with safe errors', async ({ page }) => {
  const effects = await localPage(page, '/#passport');
  const input = page.getByLabel('Choose passport file');
  for (const entry of cases.passport_negative) {
    await test.step(entry.id, async () => {
      await input.setInputFiles(upload(originalBytes));
      await expect(page.getByRole('heading', { name: original.release_id, exact: true })).toBeVisible();
      const bytes = entry.size_bytes ? Buffer.alloc(entry.size_bytes, 32)
        : Object.hasOwn(entry, 'raw') ? Buffer.from(entry.raw)
        : Buffer.from(JSON.stringify({ ...original, ...entry.patch }));
      await input.setInputFiles(upload(bytes));
      await expect(page.getByRole('alert')).toHaveText(entry.error);
      await expect(page.locator('article.preview')).toHaveCount(0);
      await expect(page.getByText('Viewer assessment: Unverified')).toHaveCount(0);
      await inert(page, effects);
    });
  }
});

test('C27 cancelled native file callbacks cannot restore old passport claims', async ({ page }) => {
  const effects = await localPage(page, '/#passport');
  const input = page.getByLabel('Choose passport file');
  await input.setInputFiles(upload(originalBytes));
  await expect(page.getByRole('heading', { name: original.release_id, exact: true })).toBeVisible();
  await page.evaluate(() => {
    const read = FileReader.prototype.readAsText;
    const pending: (() => Promise<void>)[] = [];
    FileReader.prototype.readAsText = function (...args) {
      pending.push(() => new Promise<void>(resolve => {
        this.addEventListener('loadend', () => resolve(), { once: true });
        read.apply(this, args);
      }));
    };
    Object.defineProperty(window, '__c27FlushReads', { value: async () => {
      for (const resume of pending.splice(0)) await resume();
    } });
  });
  await input.setInputFiles(upload(passportBytes));
  await expect(page.locator('article.preview')).toHaveCount(0);
  await page.getByRole('button', { name: 'Cancel reading' }).click();
  await page.evaluate(() => Reflect.get(window, '__c27FlushReads')());
  await expect(page.getByRole('status')).toHaveText('File reading cancelled.');
  await expect(page.locator('article.preview')).toHaveCount(0);
  await inert(page, effects);
  // The same selected file remains usable after cancellation.
  await input.setInputFiles(upload(originalBytes));
  await page.evaluate(() => Reflect.get(window, '__c27FlushReads')());
  await expect(page.getByRole('heading', { name: original.release_id, exact: true })).toBeVisible();
});

test('C27 forged headers cannot open disabled object/provider APIs or turn health into a writer', async ({ request }) => {
  for (const entry of cases.http_negative) {
    await test.step(entry.id, async () => {
      const response = await request.fetch(entry.path, {
        method: entry.method, headers: entry.headers,
        ...(entry.method === 'POST' ? { data: { owner_id: 'another-owner', source: 'C27_INERT_SOURCE' } } : {}),
      });
      expect(response.status()).toBe(entry.status);
      expect(await response.json()).toEqual({ error: entry.error });
      expect(response.headers()['cache-control']).toBe('no-store');
      if (entry.status === 405) expect(response.headers().allow).toBe('GET, HEAD');
    });
  }
  const health = await request.get('/api/engine/health');
  expect(health.status()).toBe(200);
  expect(await health.json()).toMatchObject({ schemaVersion: 1, status: 'ok', scope: 'engine-process' });
});
