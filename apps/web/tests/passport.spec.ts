import { expect, test } from '@playwright/test';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';

const fixture = new URL('../../../tests/fixtures/attestara/C21/positive.json', import.meta.url);
const negative = JSON.parse(await readFile(new URL('../../../tests/fixtures/attestara/C21/negative.json', import.meta.url), 'utf8'));
const expected = JSON.parse(await readFile(new URL('../../../tests/fixtures/attestara/C21/expected.json', import.meta.url), 'utf8'));
const redaction = JSON.parse(await readFile(new URL('../../../tests/fixtures/attestara/C25/cases.json', import.meta.url), 'utf8'));

test('C21 synthetic fixture hashes match their provenance', async () => {
  const root = new URL('../../../tests/fixtures/attestara/C21/', import.meta.url);
  const provenance = JSON.parse(await readFile(new URL('provenance.json', root), 'utf8'));
  expect(provenance.license).toBe('Apache-2.0');
  for (const [name, hash] of Object.entries(provenance.files)) {
    expect('sha256:' + createHash('sha256').update(await readFile(new URL(name, root))).digest('hex')).toBe(hash);
  }
});

test('local C19 passport shows policy and evidence without uploading its bytes', async ({ page }) => {
  const requests: string[] = [];
  page.on('request', request => { if (request.method() !== 'GET') requests.push(request.url()); });
  await page.goto('/#passport');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Inspect a release passport.');
  await page.getByLabel('Choose passport file').setInputFiles({ name: 'passport.json', mimeType: 'application/json', buffer: await readFile(fixture) });
  await expect(page.getByRole('heading', { name: expected.release_id })).toBeVisible();
  await expect(page.getByText(expected.viewer_assessment)).toBeVisible();
  await expect(page.getByText(expected.evidence_path)).toBeVisible();
  await expect(page.getByText(expected.policy_decision)).toBeVisible();
  await expect(page.getByRole('table')).toBeVisible();
  await page.setViewportSize({ width: 360, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(requests).toEqual([]);
});

test('viewer presents the C25 public redaction marker without exposing source text', async ({ page }) => {
  const passport = JSON.parse(await readFile(fixture, 'utf8'));
  const sensitive = redaction.cases.find((entry: { id: string }) => entry.id === 'bearer');
  expect(sensitive).toBeTruthy();
  passport.release_id = sensitive.expected;
  passport.source_revision = sensitive.expected;
  await page.goto('/#passport');
  await page.getByLabel('Choose passport file').setInputFiles({ name: 'redacted.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(passport)) });
  await expect(page.getByRole('heading', { name: '[REDACTED]' })).toBeVisible();
  await expect(page.getByText(sensitive.value)).toHaveCount(0);
  await expect(page.getByRole('table')).toBeVisible();
});

test('invalid, unsupported and oversized passports show an error', async ({ page }) => {
  await page.goto('/#passport');
  await page.setViewportSize({ width: 360, height: 800 });
  const input = page.getByLabel('Choose passport file');
  await expect(page.getByText('No passport selected.')).toBeVisible();
  await input.setInputFiles({ name: 'empty.json', mimeType: 'application/json', buffer: Buffer.alloc(0) });
  await expect(page.getByRole('alert')).toContainText('empty');
  await input.setInputFiles({ name: 'broken.json', mimeType: 'application/json', buffer: Buffer.from(negative.malformed_json) });
  await expect(page.getByRole('alert')).toContainText('valid JSON');
  await input.setInputFiles({ name: 'future.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(negative.unsupported_version)) });
  await expect(page.getByRole('alert')).toContainText('version 1');
  const duplicate = JSON.parse(await readFile(fixture, 'utf8'));
  duplicate.evidence_references.push(duplicate.evidence_references[0]);
  await input.setInputFiles({ name: 'duplicate.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(duplicate)) });
  await expect(page.getByRole('alert')).toContainText('valid version 1');
  await input.setInputFiles({ name: 'large.json', mimeType: 'application/json', buffer: Buffer.alloc(1048577, 32) });
  await expect(page.getByRole('alert')).toContainText('preview limit');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('unknown policy claims remain unverified', async ({ page }) => {
  await page.goto('/#passport');
  const passport = JSON.parse(await readFile(fixture, 'utf8'));
  passport.policy_decision = 'unknown';
  await page.getByLabel('Choose passport file').setInputFiles({ name: 'unknown.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(passport)) });
  await expect(page.getByText('Claimed policy decision: unknown')).toBeVisible();
  await expect(page.getByText(expected.viewer_assessment)).toBeVisible();
});

test('cancellation, inert text, keyboard access and narrow layout remain explicit', async ({ page }) => {
  await page.addInitScript(() => {
    const nativeRead = FileReader.prototype.readAsText;
    FileReader.prototype.readAsText = function (...args) {
      setTimeout(() => { if (this.readyState === FileReader.EMPTY) nativeRead.apply(this, args); }, 250);
    };
  });
  await page.goto('/#passport');
  await page.setViewportSize({ width: 360, height: 800 });
  const input = page.getByLabel('Choose passport file');
  await input.focus();
  await expect(input).toBeFocused();
  await input.setInputFiles({ name: 'passport.json', mimeType: 'application/json', buffer: await readFile(fixture) });
  await page.getByRole('button', { name: 'Cancel reading' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status')).toContainText('File reading cancelled.');
  await page.waitForTimeout(350);
  await expect(page.getByRole('heading', { name: 'attestara-c14-release' })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('HTML-like passport text stays inert and engine unavailability is visible', async ({ page }) => {
  await page.route('**/api/engine/health', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }));
  await page.goto('/#passport');
  await page.setViewportSize({ width: 360, height: 800 });
  await expect(page.getByLabel('Availability')).toContainText('Local engine was unavailable');
  const passport = JSON.parse(await readFile(fixture, 'utf8'));
  passport.warnings = [negative.html_sentinel];
  await page.getByLabel('Choose passport file').setInputFiles({ name: 'inert.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(passport)) });
  await expect(page.getByText(passport.warnings[0])).toBeVisible();
  expect(await page.evaluate(() => Reflect.get(window, '__attestaraInjected'))).toBeUndefined();
  expect(await page.locator('img').count()).toBe(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
