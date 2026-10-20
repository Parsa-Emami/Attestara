import { expect, test, type Locator, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { cp, mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { engineCommand, runWorkflow, restoreWorkflow } from '../../scripts/clone-workflow.mjs';

const root = fileURLToPath(new URL('../../', import.meta.url));
const fixtures = join(root, 'tests/fixtures/attestara');
const cases = JSON.parse(await readFile(join(fixtures, 'C33/cases.json'), 'utf8'));
const previous = await readFile(join(fixtures, 'C21/positive.json'));
const previousPassport = JSON.parse(previous.toString());

async function tabTo(page: Page, target: Locator) {
  for (let count = 0; count < 12; count++) {
    if (await target.evaluate(element => element === document.activeElement)) return;
    await page.keyboard.press('Tab');
  }
  await expect(target).toBeFocused();
}

async function choose(page: Page, bytes: Buffer) {
  const input = page.getByLabel('Choose passport file');
  await tabTo(page, input);
  await expect(page.locator('.file-button')).toHaveCSS('outline-style', 'solid');
  const chooser = page.waitForEvent('filechooser');
  await input.press('Space');
  await (await chooser).setFiles({ name: 'c33-passport.json', mimeType: 'application/json', buffer: bytes });
}

async function reflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test('C33 versioned scenarios retain the exact existing N-1 fixture bytes', async () => {
  const provenance = JSON.parse(await readFile(join(fixtures, 'C33/provenance.json'), 'utf8'));
  expect(cases.schema_version).toBe(1);
  expect(cases.widths).toEqual([360, 1440]);
  expect(provenance.license).toBe('Apache-2.0');
  expect(Object.keys(provenance.files).sort()).toEqual([
    'tests/fixtures/attestara/C21/positive.json',
    'tests/fixtures/attestara/C09/positive.zip',
    'tests/fixtures/attestara/C33/README.md',
    'tests/fixtures/attestara/C33/cases.json',
    'tests/fixtures/attestara/C30/provenance.json',
  ].sort());
  for (const [path, hash] of Object.entries(provenance.files)) {
    expect('sha256:' + createHash('sha256').update(await readFile(join(root, path))).digest('hex')).toBe(hash);
  }
});

for (const width of cases.widths) {
  test.describe(`C33 ${width}px`, () => {
    test.use({ viewport: { width, height: 900 } });

    test('real CLI workflow, snapshot restore and N-1 files remain keyboard/table accessible', async ({ page }) => {
      const work = await mkdtemp(join(tmpdir(), 'attestara-c33-'));
      try {
        // Reuse all ten C30 actions, including actual signature, ZIP and replay/conflict negatives.
        const state = runWorkflow(root, work);
        const restored = restoreWorkflow(root, work, state);
        const oldAssessment = JSON.parse(engineCommand(root, ['verify', '--archive', join(fixtures, 'C09/positive.zip')]).stdout);
        expect(oldAssessment.file_integrity).toBe('verified');
        expect(oldAssessment.manifest_trusted).toBe(false);
        expect(JSON.parse(engineCommand(root, ['verify', '--passport', join(fixtures, 'C21/positive.json')]).stdout)).toEqual(previousPassport);

        const c30 = join(fixtures, 'C30');
        const stale = engineCommand(root, ['evaluate', '--input', join(c30, 'manifest.json'), '--policy', join(c30, 'rule.json'), '--evidence', join(c30, 'evidence.json'), '--freshness', 'stale'], 1);
        expect(JSON.parse(stale.stdout).policy_decision).toBe(cases.stale_decision);
        const missing = join(work, 'missing');
        await cp(c30, missing, { recursive: true });
        await rm(join(missing, 'evidence.json'));
        const missingResult = engineCommand(root, ['verify', '--input', join(c30, 'manifest.json'), '--files', missing], 3);
        expect(missingResult.stderr).toContain('input_unavailable');
        expect(engineCommand(root, ['verify', '--store', state.store, '--operation-id', 'release-1']).stdout).toBe(state.first);

        await page.goto('/#passport');
        await expect(page.getByLabel('Availability')).toContainText('healthy at the last check');
        await page.keyboard.press('Tab');
        await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused();
        await page.keyboard.press('Enter');
        await expect(page.locator('main#content')).toBeFocused();
        await expect(page).toHaveURL(/#passport$/);
        const effects: string[] = [];
        page.on('request', request => {
          if (request.method() === 'GET' && request.url().endsWith('/icon.svg') && request.postData() === null) return;
          effects.push(`${request.method()} ${request.url()}`);
        });
        page.on('pageerror', error => effects.push(error.message));
        for (const bytes of [Buffer.from(state.first), Buffer.from(restored), previous, Buffer.from(stale.stdout)]) {
          const passport = JSON.parse(bytes.toString());
          await choose(page, bytes);
          await expect(page.locator('article.preview h3').first()).toHaveText(passport.release_id);
          await expect(page.getByText(`Claimed policy decision: ${passport.policy_decision}`)).toBeVisible();
          await expect(page.getByText(cases.viewer_assessment)).toBeVisible();
          await expect(page.getByRole('table').getByRole('columnheader')).toHaveText(['Path', 'Claimed digest']);
          const rows = page.getByRole('table').locator('tbody tr');
          await expect(rows).toHaveCount(passport.evidence_references.length);
          for (const [index, reference] of passport.evidence_references.entries()) {
            await expect(rows.nth(index).getByRole('cell')).toHaveText([reference.path, reference.digest]);
          }
          await reflow(page);
          await tabTo(page, page.getByRole('button', { name: 'Clear passport' }));
          await page.keyboard.press('Enter');
          await expect(page.getByRole('status')).toHaveText('No passport selected.');
          await expect(page.locator('article.preview')).toHaveCount(0);
        }
        expect(effects).toEqual([]);
      } finally { await rm(work, { recursive: true, force: true }); }
    });

    test('empty/invalid input discards prior claims and remains usable during engine outage', async ({ page }) => {
      // HTTP fault injection tests the UI projection; real provider failures run in the Go C15 suite.
      await page.route('**/api/engine/health', route => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }));
      await page.goto('/#passport');
      await expect(page.getByLabel('Availability')).toContainText('Local engine was unavailable');
      await expect(page.getByLabel('Availability')).toHaveAttribute('aria-live', 'polite');
      await expect(page.getByRole('status')).toHaveText('No passport selected.');
      for (const entry of cases.invalid) {
        await choose(page, previous);
        await expect(page.locator('article.preview')).toBeVisible();
        await choose(page, Buffer.from(entry.text));
        await expect(page.getByRole('alert')).toHaveText(entry.error);
        await expect(page.locator('article.preview')).toHaveCount(0);
        await reflow(page);
      }
      const hostile = { ...previousPassport, release_id: cases.html_sentinel, warnings: [cases.html_sentinel] };
      await choose(page, Buffer.from(JSON.stringify(hostile)));
      await expect(page.locator('article.preview h3').first()).toHaveText(cases.html_sentinel);
      await expect(page.locator('article.preview li')).toHaveText([cases.html_sentinel]);
      await expect(page.getByText(cases.viewer_assessment)).toBeVisible();
      await expect(page.locator('main img, main script, main iframe')).toHaveCount(0);
      expect(await page.evaluate(() => Reflect.get(window, '__c33Injected'))).toBeUndefined();
      await reflow(page);
    });

    test('keyboard cancellation suppresses late native reads and allows the same file again', async ({ page }) => {
      await page.goto('/#passport');
      await choose(page, previous);
      await expect(page.locator('article.preview')).toBeVisible();
      await page.evaluate(() => {
        const nativeRead = FileReader.prototype.readAsText;
        const pending: (() => Promise<void>)[] = [];
        FileReader.prototype.readAsText = function (...args) {
          pending.push(() => new Promise<void>(resolve => {
            this.addEventListener('loadend', () => resolve(), { once: true });
            nativeRead.apply(this, args);
          }));
        };
        Object.defineProperty(window, '__c33Flush', { value: async () => { for (const read of pending.splice(0)) await read(); } });
      });
      await choose(page, previous);
      await expect(page.getByRole('status')).toHaveText('Reading local passport…');
      await expect(page.locator('article.preview')).toHaveCount(0);
      await tabTo(page, page.getByRole('button', { name: 'Cancel reading' }));
      await page.keyboard.press('Enter');
      await page.evaluate(() => Reflect.get(window, '__c33Flush')());
      await expect(page.getByRole('status')).toHaveText('File reading cancelled.');
      await expect(page.locator('article.preview')).toHaveCount(0);
      await choose(page, previous);
      await page.evaluate(() => Reflect.get(window, '__c33Flush')());
      await expect(page.locator('article.preview h3').first()).toHaveText(previousPassport.release_id);
      await reflow(page);
    });
  });
}
