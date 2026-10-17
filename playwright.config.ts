import { defineConfig } from '@playwright/test';
const port = process.env.WEB_TEST_PORT ?? '4173';
const baseURL = `http://127.0.0.1:${port}`;
export default defineConfig({
  testDir: './tests/browser',
  timeout: 30000,
  use: {
    baseURL, browserName: 'chromium', headless: true,
    launchOptions: process.env.WEB_TEST_CHROMIUM ? { executablePath: process.env.WEB_TEST_CHROMIUM, args: ['--no-sandbox'] } : {},
  },
  webServer: { command: `npm start -- --port ${port}`, url: `${baseURL}/readyz`, reuseExistingServer: false, timeout: 15000 },
  reporter: 'list',
});
