import { defineConfig } from '@playwright/test';
const port = process.env.WEB_TEST_PORT ?? '4173';
const baseURL = `http://127.0.0.1:${port}`;
export default defineConfig({
  testDir: '.',
  testMatch: ['tests/browser/**/*.spec.ts', 'apps/web/tests/**/*.spec.ts'],
  timeout: 30000,
  use: {
    baseURL, browserName: 'chromium', headless: true, trace: 'retain-on-failure',
    launchOptions: process.env.WEB_TEST_CHROMIUM ? { executablePath: process.env.WEB_TEST_CHROMIUM, args: ['--no-sandbox'] } : {},
  },
  webServer: process.env.C30_EXTERNAL_SERVER ? undefined : { command: `npm start -- --port ${port}`, url: `${baseURL}/readyz`, reuseExistingServer: false, timeout: 15000 },
  reporter: 'list',
});
