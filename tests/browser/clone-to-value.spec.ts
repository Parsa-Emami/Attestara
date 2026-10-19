import { test, expect } from '@playwright/test';
import { mkdtemp, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runWorkflow, restoreWorkflow } from '../../scripts/clone-workflow.mjs';

// The CLI produces and restores the passport; the viewer never receives a mock result.
test('C30 real CLI snapshot and restored passport agree with the local viewer', async ({ page }) => {
  const root = fileURLToPath(new URL('../../', import.meta.url));
  const work = await mkdtemp(join(tmpdir(), 'attestara-c30-browser-'));
  try {
    let passports: string[];
    if (process.env.C30_ACCEPTANCE_PASSPORT) {
      passports = [await readFile(process.env.C30_ACCEPTANCE_PASSPORT, 'utf8')];
    } else {
      const state = runWorkflow(root, work);
      passports = [state.first, restoreWorkflow(root, work, state)];
    }
    const writes: string[] = [];
    page.on('request', request => { if (request.method() !== 'GET') writes.push(request.url()); });
    await page.goto('/#passport');
    for (const text of passports) {
      const passport = JSON.parse(text);
      await page.getByLabel('Choose passport file').setInputFiles({ name: 'c30-passport.json', mimeType: 'application/json', buffer: Buffer.from(text) });
      await expect(page.getByRole('heading', { name: passport.release_id })).toBeVisible();
      await expect(page.getByText(`Claimed policy decision: ${passport.policy_decision}`)).toBeVisible();
      await expect(page.getByText(passport.subject_digest, { exact: true })).toBeVisible();
      await expect(page.getByText(passport.report_digest, { exact: true })).toBeVisible();
      await expect(page.getByText('Viewer assessment: Unverified')).toBeVisible();
    }
    expect(writes).toEqual([]);
  } finally { await rm(work, { recursive: true, force: true }); }
});
